package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"kun-galgame-api/internal/middleware"
	"kun-galgame-api/internal/user/dto"
	"kun-galgame-api/internal/user/oauth"
	"kun-galgame-api/internal/user/repository"
	"kun-galgame-api/pkg/errors"
	"kun-galgame-api/pkg/role"
	"kun-galgame-api/pkg/userclient"

	"github.com/redis/go-redis/v9"
)

// AuthService handles the OAuth-callback session bootstrap and a few
// session-lifecycle ops. Identity (name / avatar / email / bio / status)
// is owned by OAuth — kungal only persists per-site state in
// kungal_user_state (moemoepoint, daily counters).
type AuthService struct {
	stateRepo   *repository.StateRepository
	rdb         *redis.Client
	oauthClient *oauth.Client
	userClient  *userclient.Client
}

func NewAuthService(
	stateRepo *repository.StateRepository,
	rdb *redis.Client,
	oauthClient *oauth.Client,
	userClient *userclient.Client,
) *AuthService {
	return &AuthService{
		stateRepo:   stateRepo,
		rdb:         rdb,
		oauthClient: oauthClient,
		userClient:  userClient,
	}
}

// OAuthCallback exchanges the authorization code for tokens, fetches user
// info from OAuth (which is the single source of truth for identity), and
// creates a kungal session. Idempotently ensures kungal_user_state(userID)
// exists so the new user starts with the default 7 moemoepoint balance.
func (s *AuthService) OAuthCallback(
	ctx context.Context,
	req *dto.OAuthCallbackRequest,
) (*dto.SessionResponse, *errors.AppError) {
	tokenResp, err := s.oauthClient.ExchangeCode(req.Code, req.CodeVerifier)
	if err != nil {
		// OAuth can reject the token exchange with 10014 if the user was
		// banned between issuing the authorization code and exchanging it,
		// or on some upstream paths that allow code issuance for soon-to-be-
		// banned accounts. Surface distinctly so the frontend doesn't bounce
		// the user back through /oauth/authorize (a loop they can't break).
		if oauth.IsBanned(err) {
			return nil, errors.ErrAccountBanned()
		}
		return nil, errors.ErrBadRequest(fmt.Sprintf("OAuth 授权码交换失败: %v", err))
	}

	oauthUser, err := s.oauthClient.FetchUserInfo(tokenResp.AccessToken)
	if err != nil {
		if oauth.IsBanned(err) {
			return nil, errors.ErrAccountBanned()
		}
		return nil, errors.ErrBadRequest(fmt.Sprintf("获取 OAuth 用户信息失败: %v", err))
	}
	if oauthUser.ID <= 0 {
		// Hard-fail if the OAuth server hasn't yet been updated to include
		// the integer `id` field on /oauth/userinfo. See the comment on
		// oauth.UserInfo for the rationale.
		return nil, errors.ErrInternal(
			"OAuth /oauth/userinfo 未返回用户 id; 请确认 OAuth server 已更新",
		)
	}

	// First-time login on this site: create the kungal-state row. No-op
	// for returning users.
	if err := s.stateRepo.Ensure(oauthUser.ID); err != nil {
		return nil, errors.ErrInternal("初始化用户状态失败")
	}

	// Pull moemoepoint for the SessionResponse so the frontend can show it
	// immediately without a follow-up /user/status call.
	state, _ := s.stateRepo.FindByID(oauthUser.ID)
	moe := 0
	if state != nil {
		moe = state.Moemoepoint
	}

	// /oauth/userinfo's `picture` only carries the legacy avatar URL (empty for
	// users who set a new image_service avatar). Resolve through userclient,
	// which maps avatar_image_hash → the image_service URL, and fall back to
	// `picture` if the lookup fails — otherwise the top-bar avatar shows blank
	// right after login for anyone on a new avatar.
	avatar := oauthUser.Picture
	if u, ok, uerr := s.userClient.User(ctx, oauthUser.ID); uerr == nil && ok && u.Avatar != "" {
		avatar = u.Avatar
	}

	sessionToken, err := generateSessionToken()
	if err != nil {
		return nil, errors.ErrInternal("生成会话令牌失败")
	}

	// The login-response user the FE writes into its store. Roles is the
	// EFFECTIVE set (global ∪ this-site's site_roles, 12-site-roles §5.1); the
	// persisted session reuses the SAME slice so server enforcement and the FE
	// store can't diverge — else a site moderator is enforced server-side yet
	// never sees the moderator UI. Pure builder → the merge is unit-tested
	// (newLoginUserProfile) without a Login round-trip.
	respUser := newLoginUserProfile(oauthUser, avatar, moe)

	sessionData := middleware.SessionData{
		UserInfo: middleware.UserInfo{
			ID:    oauthUser.ID,
			Sub:   oauthUser.Sub,
			Name:  oauthUser.Name,
			Email: oauthUser.Email,
			Roles: respUser.Roles,
		},
		OAuthAccessToken:  tokenResp.AccessToken,
		OAuthRefreshToken: tokenResp.RefreshToken,
		OAuthExpiresAt:    time.Now().Unix() + int64(tokenResp.ExpiresIn),
	}

	data, err := json.Marshal(sessionData)
	if err != nil {
		return nil, errors.ErrInternal("序列化会话数据失败")
	}
	s.rdb.Set(ctx, middleware.SessionKey(sessionToken), data, middleware.SessionTTL)

	return &dto.SessionResponse{
		Token: sessionToken,
		User:  respUser,
	}, nil
}

// newLoginUserProfile builds the login-response user the FE writes into its
// store. Roles is the EFFECTIVE set = the user's global roles UNIONed with this
// site's site_roles (contract docs/oauth/12-site-roles.md §5.1) — the same set
// persisted on the session — so a site moderator/creator gets BOTH server
// enforcement and the matching UI. Pure so the merge is unit-tested without the
// OAuth / Redis / DB round-trips the full Login needs.
func newLoginUserProfile(u *oauth.UserInfo, avatar string, moe int) *dto.UserProfile {
	return &dto.UserProfile{
		ID:          u.ID,
		Sub:         u.Sub,
		Name:        u.Name,
		Avatar:      avatar,
		Roles:       role.Union(u.Roles, u.SiteRoles),
		Moemoepoint: moe,
		Bio:         "", // bio is OAuth-owned, available via /auth/me
	}
}

// Logout deletes the session from Redis and revokes the OAuth token.
func (s *AuthService) Logout(ctx context.Context, sessionToken string) error {
	val, err := s.rdb.Get(ctx, middleware.SessionKey(sessionToken)).Result()
	if err == nil {
		var session middleware.SessionData
		if json.Unmarshal([]byte(val), &session) == nil && session.OAuthRefreshToken != "" {
			_ = s.oauthClient.RevokeToken(session.OAuthRefreshToken)
		}
	}
	return s.rdb.Del(ctx, middleware.SessionKey(sessionToken)).Err()
}

// GetProfile returns the full profile for the currently logged-in user.
// Identity fields come from OAuth (via userclient); moemoepoint and other
// per-site state come from kungal_user_state. Used by GET /api/auth/me.
func (s *AuthService) GetProfile(
	ctx context.Context,
	userID int,
) (*dto.UserProfile, *errors.AppError) {
	u, ok, err := s.userClient.User(ctx, userID)
	if err != nil {
		return nil, errors.ErrInternal("查询用户信息失败")
	}
	if !ok {
		return nil, errors.ErrNotFound("用户不存在")
	}
	state, _ := s.stateRepo.FindByID(userID)
	moe := 0
	if state != nil {
		moe = state.Moemoepoint
	}
	return &dto.UserProfile{
		ID:          u.ID,
		Name:        u.Name,
		Avatar:      u.Avatar,
		Roles:       u.Roles,
		Moemoepoint: moe,
		Bio:         u.Bio,
		// Sub isn't in the userclient brief — the /auth/me handler fills it from
		// the session identity.
	}, nil
}

func generateSessionToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
