package handler

// ProfileHandler proxies "edit my profile" actions to the OAuth server
// per the代理模式 documented in docs/oauth/02-user-profile.md. bio /
// username / avatar are broker-able by kungal again because OAuth
// exposes per-field endpoints scoped to the requester's own access
// token. ban / delete stay admin-side in OAuth.
//
// All three handlers:
//  1. require a logged-in kungal session (the auth middleware on the
//     route group fires before this);
//  2. pull the session-stored OAuth access token via
//     middleware.GetAccessToken — same source as the galgame proxy;
//  3. forward to the corresponding /auth/me endpoint;
//  4. invalidate the kungal-side userclient cache on success so the
//     updated identity surfaces immediately in subsequent renders
//     (avatar / name show up in newly-loaded comments, etc.);
//  5. surface OAuth's biz code + message verbatim on failure so the
//     browser keeps the same `{code, message}` UX as before.
import (
	stderrors "errors"
	"encoding/json"
	"fmt"

	"kun-galgame-api/internal/middleware"
	"kun-galgame-api/internal/user/dto"
	"kun-galgame-api/internal/user/oauth"
	"kun-galgame-api/pkg/errors"
	"kun-galgame-api/pkg/response"
	"kun-galgame-api/pkg/userclient"
	"kun-galgame-api/pkg/utils"

	"github.com/gofiber/fiber/v3"
)

type ProfileHandler struct {
	oauthClient *oauth.Client
	userClient  *userclient.Client
}

func NewProfileHandler(oauthClient *oauth.Client, userClient *userclient.Client) *ProfileHandler {
	return &ProfileHandler{oauthClient: oauthClient, userClient: userClient}
}

// UpdateBio updates the authenticated user's bio.
// PUT /api/user/bio body {bio}
func (h *ProfileHandler) UpdateBio(c fiber.Ctx) error {
	user, appErr := middleware.MustGetUser(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	var req dto.UpdateBioRequest
	if appErr := utils.ParseAndValidate(c, &req); appErr != nil {
		return response.Error(c, appErr)
	}

	data, err := h.callPatchAuthMe(c, map[string]any{"bio": req.Bio})
	if err != nil {
		return response.Error(c, err)
	}
	h.userClient.Invalidate(user.ID)
	return response.OK(c, json.RawMessage(data))
}

// UpdateUsername updates the authenticated user's display name. OAuth
// calls it `name`; kungal historically called it `username`. The
// handler translates so the frontend can keep its existing wire shape.
// PUT /api/user/username body {username}
func (h *ProfileHandler) UpdateUsername(c fiber.Ctx) error {
	user, appErr := middleware.MustGetUser(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	var req dto.UpdateUsernameRequest
	if appErr := utils.ParseAndValidate(c, &req); appErr != nil {
		return response.Error(c, appErr)
	}

	data, err := h.callPatchAuthMe(c, map[string]any{"name": req.Username})
	if err != nil {
		return response.Error(c, err)
	}
	h.userClient.Invalidate(user.ID)
	return response.OK(c, json.RawMessage(data))
}

// UploadAvatar forwards a multipart upload to OAuth's one-step avatar
// endpoint. OAuth writes the resulting hash into the user row itself,
// so no second PATCH is needed.
// POST /api/user/avatar multipart {file}
func (h *ProfileHandler) UploadAvatar(c fiber.Ctx) error {
	user, appErr := middleware.MustGetUser(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}

	contentType := c.Get("Content-Type")
	body := c.Body()
	if len(body) == 0 || contentType == "" {
		return response.Error(c, errors.ErrBadRequest("缺少文件"))
	}

	token := middleware.GetAccessToken(c)
	if token == "" {
		return response.Error(c, errors.ErrAuthExpired())
	}

	data, err := h.oauthClient.UploadAvatar(token, body, contentType)
	if err != nil {
		return response.Error(c, mapOAuthError(err))
	}
	h.userClient.Invalidate(user.ID)
	return response.OK(c, json.RawMessage(data))
}

// callPatchAuthMe is the shared post-validation path for UpdateBio /
// UpdateUsername. Pulls the access token, calls the OAuth client, and
// hands errors back to the caller already typed as *errors.AppError.
func (h *ProfileHandler) callPatchAuthMe(c fiber.Ctx, body map[string]any) (json.RawMessage, *errors.AppError) {
	token := middleware.GetAccessToken(c)
	if token == "" {
		return nil, errors.ErrAuthExpired()
	}
	data, err := h.oauthClient.PatchAuthMe(token, body)
	if err != nil {
		return nil, mapOAuthError(err)
	}
	return data, nil
}

// mapOAuthError translates an *oauth.Error into the kungal AppError
// shape so the response keeps its standard {code, message, data}
// envelope. OAuth's envelope code is forwarded verbatim — that way
// browser-side handling (10001 → re-login, 10007 → "name taken", …)
// continues to work without an extra translation layer.
func mapOAuthError(err error) *errors.AppError {
	var oe *oauth.Error
	if stderrors.As(err, &oe) {
		if oe.Code != 0 {
			return errors.New(oe.Code, oe.Message, oe.HTTPStatus)
		}
		// Non-envelope failure (network etc.) — fall through to a
		// generic internal error so the response carries SOMETHING
		// readable instead of an opaque transport error.
		return errors.ErrInternal(fmt.Sprintf("OAuth 服务不可达: %v", err))
	}
	return errors.ErrInternal("更新失败")
}
