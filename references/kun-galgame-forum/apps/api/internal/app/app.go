package app

import (
	"context"
	"log/slog"
	"time"

	activityHandler "kun-galgame-api/internal/activity/handler"
	activityRepo "kun-galgame-api/internal/activity/repository"
	activityService "kun-galgame-api/internal/activity/service"
	adminHandler "kun-galgame-api/internal/admin/handler"
	adminRepo "kun-galgame-api/internal/admin/repository"
	adminService "kun-galgame-api/internal/admin/service"
	communitytrust "kun-galgame-api/internal/community/trust"
	docHandler "kun-galgame-api/internal/doc/handler"
	docRepo "kun-galgame-api/internal/doc/repository"
	docService "kun-galgame-api/internal/doc/service"
	friendHandler "kun-galgame-api/internal/friendlink/handler"
	friendRepo "kun-galgame-api/internal/friendlink/repository"
	"kun-galgame-api/internal/galgame/client"
	galgameHandler "kun-galgame-api/internal/galgame/handler"
	galgameRepo "kun-galgame-api/internal/galgame/repository"
	galgameService "kun-galgame-api/internal/galgame/service"
	homeHandler "kun-galgame-api/internal/home/handler"
	homeRepo "kun-galgame-api/internal/home/repository"
	homeService "kun-galgame-api/internal/home/service"
	imageHandler "kun-galgame-api/internal/image/handler"
	imageRepo "kun-galgame-api/internal/image/repository"
	imageService "kun-galgame-api/internal/image/service"
	"kun-galgame-api/internal/infrastructure/cache"
	cronPkg "kun-galgame-api/internal/infrastructure/cron"
	"kun-galgame-api/internal/infrastructure/database"
	"kun-galgame-api/internal/infrastructure/mail"
	"kun-galgame-api/internal/infrastructure/markdown"
	"kun-galgame-api/internal/infrastructure/storage"
	msgHandler "kun-galgame-api/internal/message/handler"
	msgRepo "kun-galgame-api/internal/message/repository"
	msgService "kun-galgame-api/internal/message/service"
	"kun-galgame-api/internal/moemoepoint"
	rankingHandler "kun-galgame-api/internal/ranking/handler"
	rankingRepo "kun-galgame-api/internal/ranking/repository"
	rankingService "kun-galgame-api/internal/ranking/service"
	rssHandler "kun-galgame-api/internal/rss/handler"
	rssRepo "kun-galgame-api/internal/rss/repository"
	searchHandler "kun-galgame-api/internal/search/handler"
	searchRepo "kun-galgame-api/internal/search/repository"
	searchService "kun-galgame-api/internal/search/service"
	sectionHandler "kun-galgame-api/internal/section/handler"
	sectionRepo "kun-galgame-api/internal/section/repository"
	sectionService "kun-galgame-api/internal/section/service"
	toolsetHandler "kun-galgame-api/internal/toolset/handler"
	toolsetRepo "kun-galgame-api/internal/toolset/repository"
	toolsetService "kun-galgame-api/internal/toolset/service"
	topicHandler "kun-galgame-api/internal/topic/handler"
	topicRepo "kun-galgame-api/internal/topic/repository"
	topicService "kun-galgame-api/internal/topic/service"
	"kun-galgame-api/internal/trust/enforce"
	"kun-galgame-api/internal/trust/gate"
	trustHandler "kun-galgame-api/internal/trust/handler"
	trustService "kun-galgame-api/internal/trust/service"
	updateHandler "kun-galgame-api/internal/update/handler"
	updateRepo "kun-galgame-api/internal/update/repository"
	"kun-galgame-api/internal/user/handler"
	"kun-galgame-api/internal/user/oauth"
	"kun-galgame-api/internal/user/repository"
	"kun-galgame-api/internal/user/service"
	websiteHandler "kun-galgame-api/internal/website/handler"
	websiteRepo "kun-galgame-api/internal/website/repository"
	websiteService "kun-galgame-api/internal/website/service"
	"kun-galgame-api/pkg/artifactclient"
	"kun-galgame-api/pkg/catalogclient"
	"kun-galgame-api/pkg/communityclient"
	"kun-galgame-api/pkg/config"
	"kun-galgame-api/pkg/dlsite"
	"kun-galgame-api/pkg/errors"
	"kun-galgame-api/pkg/imageclient"
	"kun-galgame-api/pkg/linkcheck"
	"kun-galgame-api/pkg/response"
	"kun-galgame-api/pkg/trustclient"
	"kun-galgame-api/pkg/userclient"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type App struct {
	Fiber       *fiber.App
	DB          *gorm.DB
	Redis       *redis.Client
	Mailer      *mail.Mailer
	Config      *config.Config
	OAuthClient *oauth.Client
	UserState   *repository.StateRepository
	UserClient  *userclient.Client

	// Handlers
	OAuthHandler                   *handler.OAuthHandler
	UserHandler                    *handler.UserHandler
	UserProfileHandler             *handler.ProfileHandler
	HomeHandler                    *homeHandler.HomeHandler
	TopicHandler                   *topicHandler.TopicHandler
	TopicDraftHandler              *topicHandler.TopicDraftHandler
	ReplyHandler                   *topicHandler.ReplyHandler
	TopicCommentHandler            *topicHandler.CommentHandler
	PollHandler                    *topicHandler.PollHandler
	MessageHandler                 *msgHandler.MessageHandler
	MessageChatHandler             *msgHandler.ChatHandler
	AdminOverviewHandler           *adminHandler.OverviewHandler
	AdminPurgeHandler              *adminHandler.PurgeHandler
	AdminRolePermissionHandler     *adminHandler.RolePermissionHandler
	AdminUserPermissionHandler     *adminHandler.UserPermissionHandler
	AdminPermissionAuditHandler    *adminHandler.PermissionAuditHandler
	RankingHandler                 *rankingHandler.RankingHandler
	SectionHandler                 *sectionHandler.SectionHandler
	DocArticleHandler              *docHandler.ArticleHandler
	DocCategoryHandler             *docHandler.CategoryHandler
	DocTagHandler                  *docHandler.TagHandler
	WebsiteHandler                 *websiteHandler.WebsiteHandler
	WebsiteCategoryHandler         *websiteHandler.CategoryHandler
	WebsiteTagHandler              *websiteHandler.TagHandler
	UpdateHandler                  *updateHandler.UpdateHandler
	FriendLinkHandler              *friendHandler.FriendLinkHandler
	TrustHandler                   *trustHandler.TrustHandler
	RSSHandler                     *rssHandler.RSSHandler
	GalgameHandler                 *galgameHandler.GalgameHandler
	GalgameCollectionHandler       *galgameHandler.GalgameCollectionHandler
	GalgameCommunityCommentHandler *galgameHandler.CommunityCommentHandler
	ResourceCommentHandler         *galgameHandler.ResourceCommentHandler
	GalgameResourceHandler         *galgameHandler.ResourceHandler
	GalgameRatingHandler           *galgameHandler.RatingHandler
	GalgameQuizHandler             *galgameHandler.QuizHandler
	CreatorHandler                 *galgameHandler.CreatorHandler
	GalgameEntityHandler           *galgameHandler.EntityHandler
	GalgameCalendarHandler         *galgameHandler.CalendarHandler
	GalgameDraftsHandler           *galgameHandler.DraftsHandler
	GalgameProxyHandler            *galgameHandler.GalgameProxyHandler
	GalgameSubmissionHandler       *galgameHandler.SubmissionHandler
	GalgameClaimReviewHandler      *galgameHandler.ClaimReviewHandler
	GalgameEditHandler             *galgameHandler.EditHandler
	ActivityHandler                *activityHandler.ActivityHandler
	ImageHandler                   *imageHandler.ImageHandler
	SearchHandler                  *searchHandler.SearchHandler
	ToolsetHandler                 *toolsetHandler.ToolsetHandler
	ToolsetPracticalityHandler     *toolsetHandler.PracticalityHandler
	ToolsetResourceHandler         *toolsetHandler.ResourceHandler
	ToolsetUploadHandler           *toolsetHandler.UploadHandler
	CronStop                       func()
	RolePermStop                   func()
}

func New(cfg *config.Config) *App {
	// Infrastructure
	db := database.NewPostgres(cfg.Database, cfg.Server.Mode)
	rdb := cache.NewRedis(cfg.Redis)
	// fileStorageClient: archive storage (B2). Toolset .7z/.zip/.rar uploads
	// via presigned URLs, configured via FILE_STORAGE_* env vars. This is the
	// only S3-API bucket left — inline / content images all go through
	// image_service now, so there is no separate image bed (R2) client.
	fileStorageClient := storage.NewS3(cfg.FileStorage)
	if fileStorageClient == nil {
		slog.Warn("FILE_STORAGE_* 未配置, 工具集上传将不可用")
	}
	mailer := mail.NewMailer(cfg.Mail)

	// Resolve /image/<hash> content refs to absolute CDN URLs at render time.
	// Same CDN base the image client / galgame banner walker use.
	markdown.SetContentImageCDNBase(cfg.NextMoeAPI.ImageCDNBase)

	// Repositories
	userStateRepo := repository.NewStateRepository(db)
	userStatsRepo := repository.NewUserStatsRepository(db)
	userContentRepo := repository.NewUserContentRepository(db)
	messageRepository := msgRepo.NewMessageRepository(db)
	chatRepository := msgRepo.NewChatRepository(db)

	// Galgame catalog client (shared — user service needs it too). Single
	// /v1 face with the service X-API-Key since wave 169 (the wiki's
	// internal/legacy bases retired with it).
	gc := client.New(
		cfg.NextMoeAPI.BaseURL,
		cfg.NextMoeAPI.APIKey,
		cfg.NextMoeAPI.ImageCDNBase,
	)

	// OAuth client (used by auth service).
	oauthClient := oauth.NewClient(cfg.OAuth)

	// OAuth user-info client. Identity (name/avatar/bio/status/roles) is
	// owned by OAuth post-migration; mappers call this for batch enrichment.
	uc := userclient.New(userclient.Config{
		BaseURL:      cfg.OAuth.ServerURL,
		ClientID:     cfg.OAuth.ClientID,
		ClientSecret: cfg.OAuth.ClientSecret,
		// Same image_service CDN base the galgame client uses — lets userclient
		// resolve users' avatar_image_hash into URLs (new avatars store only the
		// hash; the legacy `avatar` field is empty).
		ImageCDNBase: cfg.NextMoeAPI.ImageCDNBase,
	})

	// Install the process-wide moemoepoint Awarder: OAuth is the single source
	// of truth; every change goes through it and the returned authoritative
	// balance is mirrored into the local kungal_user_state cache (no local +=).
	// See internal/moemoepoint + docs/oauth/06-moemoepoint.md.
	moemoepoint.SetDefault(moemoepoint.NewAwarder(uc, db))

	// image_service client — covers/screenshots multi-image upload path
	// (U2 / K-PR3a). ONLY construct when credentials are present, so the
	// downstream service-level `imgCli == nil` guard actually fires when
	// the operator forgot to set KUN_IMAGE_CLIENT_ID/SECRET — and
	// surfaces "图片上传服务未配置" instead of a misleading image_service
	// 401 when the user tries to upload. Mirrors the galgame side's same
	// guard pattern. A loud warn-on-startup so ops notices early.
	var imgCli *imageclient.Client
	if cfg.ImageClient.ClientID != "" && cfg.ImageClient.ClientSecret != "" {
		imgCli = imageclient.New(imageclient.Config{
			BaseURL:      cfg.ImageClient.BaseURL,
			CDNBase:      cfg.NextMoeAPI.ImageCDNBase,
			ClientID:     cfg.ImageClient.ClientID,
			ClientSecret: cfg.ImageClient.ClientSecret,
		})
		slog.Info("image_service client configured", "base_url", cfg.ImageClient.BaseURL)

		// Enrich server-rendered content <img> tags with intrinsic dims +
		// ThumbHash (no-CLS aspect-ratio reservation + blur-up). The resolver
		// caches forever and the markdown renderer calls it synchronously, so
		// warm renders touch no network. Only wired when image_service is
		// configured; otherwise content images render as plain lazy <img>.
		markdown.SetContentImageMetaResolver(imgCli.NewMetaResolver(0).Resolve)
	} else {
		slog.Warn("image_service client NOT configured; /image/galgame upload will return 未配置 — set KUN_IMAGE_CLIENT_ID / KUN_IMAGE_CLIENT_SECRET")
	}

	// artifact service client (toolset archive upload/download). kungal's OAuth
	// client IS its artifact "site", so credentials default to the OAuth client
	// when KUN_ARTIFACT_CLIENT_ID/SECRET are unset. New() degrades to a no-op
	// (calls return ErrNotConfigured) when neither is set, so a dev box without
	// artifact creds still boots.
	artClientID := cfg.ArtifactClient.ClientID
	if artClientID == "" {
		artClientID = cfg.OAuth.ClientID
	}
	artClientSecret := cfg.ArtifactClient.ClientSecret
	if artClientSecret == "" {
		artClientSecret = cfg.OAuth.ClientSecret
	}
	artCli := artifactclient.New(artifactclient.Config{
		BaseURL:      cfg.ArtifactClient.BaseURL,
		ClientID:     artClientID,
		ClientSecret: artClientSecret,
	})
	if artCli.Configured() {
		slog.Info("artifact service client configured", "base_url", cfg.ArtifactClient.BaseURL)
	} else {
		slog.Warn("artifact service client NOT configured; toolset upload will return 未配置 — set KUN_ARTIFACT_CLIENT_BASE_URL + OAuth creds")
	}

	// Trust & Safety client — report intake (Phase 1) + moderator inbox proxy
	// (Phase 3). Basic auth reuses the OAuth client_id/secret; the trust service
	// reads oauth_clients.catalog_site to derive kungal's site. Degrades to a
	// no-op when KUN_TRUST_BASE_URL / OAuth creds are unset.
	trustCli := trustclient.New(trustclient.Config{
		BaseURL:      cfg.Trust.BaseURL,
		ClientID:     cfg.OAuth.ClientID,
		ClientSecret: cfg.OAuth.ClientSecret,
	})
	if trustCli.Configured() {
		slog.Info("trust service client configured", "base_url", cfg.Trust.BaseURL)
	} else {
		slog.Warn("trust service client NOT configured; reporting returns 未启用 — set KUN_TRUST_BASE_URL + OAuth creds")
	}

	// Best-effort declarative subject-kind registration (onboarding §5). The
	// forum's COMPLETE kind universe is declared in code (gate.CanonicalSubjectKinds)
	// and self-reported to the trust registry on boot — key-only + idempotent, so
	// a re-run converges to all-`unchanged` and never clobbers admin-configured
	// callbacks. This is registration hygiene, NOT a moderation feature, so it is
	// deliberately NOT gated on KUN_TRUST_CHECK/SCAN_ENABLED — only on the client
	// being wired. Fire-and-forget in a goroutine with a short timeout: any
	// failure warns and moves on, never blocking boot.
	if trustCli.Configured() {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			results, err := trustCli.EnsureSubjectKinds(ctx, gate.CanonicalSubjectKindItems())
			if err != nil {
				slog.Warn("trust subject-kind ensure failed (non-fatal)", "err", err)
				return
			}
			// Steady state is quiet: surface only the kinds that actually changed.
			changed := make([]string, 0, len(results))
			for _, r := range results {
				if r.Result != "unchanged" {
					changed = append(changed, r.Key+"="+r.Result)
				}
			}
			if len(changed) > 0 {
				slog.Info("trust subject-kind ensure applied", "changed", changed, "total", len(results))
			} else {
				slog.Info("trust subject-kind ensure: all kinds already registered", "total", len(results))
			}
		}()
	}

	// Trust moderation gates for the forum write surface. Wave 1 = topic + reply
	// create/edit; wave 2 = the remaining user-text writes (topic comment/poll,
	// galgame rating/resource/collection/quiz/quiz-answer, toolset + its
	// resources). TWO INDEPENDENT switches, both default OFF
	// (KUN_TRUST_CHECK_ENABLED / KUN_TRUST_SCAN_ENABLED) AND gated on the trust
	// client being configured — never keyed off client presence alone, so a
	// reports-configured production forum does not auto-enable check/scan on
	// deploy. Assign the concrete client to the interface only when live, to
	// avoid a typed-nil interface. check = synchronous pre-write word-list gate
	// (deny blocks, hold publishes+logs, fail-open ≤500ms); scan = async
	// post-commit shadow scan (best-effort, no retry).
	var trustChecker gate.Checker
	if cfg.Trust.CheckEnabled && trustCli.Configured() {
		trustChecker = trustCli
	}
	trustCheck := gate.NewCheckService(trustChecker)
	if trustCheck.Enabled() {
		slog.Info("trust check gate enabled (synchronous word-list gate on all forum user-text writes)")
	} else {
		slog.Info("trust check gate disabled (KUN_TRUST_CHECK_ENABLED off or trust client unconfigured)")
	}

	var trustScanner gate.Scanner
	if cfg.Trust.ScanEnabled && trustCli.Configured() {
		trustScanner = trustCli
	}
	trustScan := gate.NewScanService(trustScanner)
	if trustScan.Enabled() {
		slog.Info("trust shadow scan enabled (async post-commit scan on all forum user-text writes)")
	} else {
		slog.Info("trust shadow scan disabled (KUN_TRUST_SCAN_ENABLED off or trust client unconfigured)")
	}

	// Catalog S2S client — the editing engine's actor-assertion face (staff/owner
	// writes + the whole review chain). Basic auth reuses the OAuth
	// client_id/secret. Degrades to a no-op (calls return ErrNotConfigured → 503)
	// when KUN_CATALOG_API_BASE / OAuth creds are unset, so a dev box without a
	// catalog service still boots.
	catalogCli := catalogclient.New(catalogclient.Config{
		BaseURL:      cfg.Catalog.BaseURL,
		ClientID:     cfg.OAuth.ClientID,
		ClientSecret: cfg.OAuth.ClientSecret,
	})
	if catalogCli.Configured() {
		slog.Info("catalog service client configured", "base_url", cfg.Catalog.BaseURL)
	} else {
		slog.Warn("catalog service client NOT configured; galgame edit review returns 未启用 — set KUN_CATALOG_API_BASE + OAuth creds")
	}

	// Community primitive client — the galgame comment area reroute (charter step
	// 03; kun-galgame-infra cmd/community :9282). Basic auth reuses the OAuth
	// client_id/secret (the community service derives kungal's tenant from
	// oauth_clients.catalog_site), so the S2S creds default to the OAuth client
	// when KUN_COMMUNITY_CLIENT_ID/SECRET are unset. Degrades to a no-op (calls
	// return ErrNotConfigured) when the base URL/creds are unset, so a dev box
	// without a community service boots even with the flag flipped on.
	commClientID := cfg.Community.ClientID
	if commClientID == "" {
		commClientID = cfg.OAuth.ClientID
	}
	commClientSecret := cfg.Community.ClientSecret
	if commClientSecret == "" {
		commClientSecret = cfg.OAuth.ClientSecret
	}
	communityCli := communityclient.New(communityclient.Config{
		BaseURL:      cfg.Community.BaseURL,
		ClientID:     commClientID,
		ClientSecret: commClientSecret,
	})
	// Community is now the unconditional comment backend (galgame + the three
	// resource areas). When the client is unconfigured, comment reads degrade to
	// empty pages and writes to 503 — a dev box without a community service still
	// boots.
	if communityCli.Configured() {
		slog.Info("community comment backend configured", "base_url", cfg.Community.BaseURL)
	} else {
		slog.Warn("community comment backend NOT configured; comments degrade (reads empty / writes 503) — set KUN_COMMUNITY_API_BASE + OAuth creds")
	}

	// DLsite affiliate 补票 link. The vendored whitelist count is logged so a
	// mis-vendored (empty / truncated) verified.tsv shows up at boot instead of
	// silently dropping the purchase button on ~1,400 games.
	if cfg.Dlsite.Configured() {
		slog.Info("dlsite affiliate link configured",
			"verified_whitelist", dlsite.VerifiedCount(),
			"coupon", cfg.Dlsite.CouponURL != "")
	} else {
		slog.Info("dlsite affiliate link off (KUN_DLSITE_LINK_TEMPLATE unset); 补票提示 renders its plain form")
	}
	// Login-time trust Boost reporter (staff/veteran). Self-gates on client
	// config, so no Boost fires when the community client is unconfigured.
	communityBooster := communitytrust.New(communityCli, rdb, db)

	// kungal-link-live-checker client — the "report resource expired" gate.
	// Only construct when BOTH base URL + API key are set; otherwise the gate is
	// nil and MarkExpired falls back to the legacy single-report-expires flow
	// (so the feature degrades safely when the checker isn't deployed).
	var linkChecker *linkcheck.Client
	if cfg.LinkChecker.BaseURL != "" && cfg.LinkChecker.APIKey != "" {
		linkChecker = linkcheck.New(linkcheck.Config{
			BaseURL:              cfg.LinkChecker.BaseURL,
			APIKey:               cfg.LinkChecker.APIKey,
			CFAccessClientID:     cfg.LinkChecker.CFAccessClientID,
			CFAccessClientSecret: cfg.LinkChecker.CFAccessClientSecret,
		})
		slog.Info("link-live-checker gate configured",
			"base_url", cfg.LinkChecker.BaseURL,
			"cf_access", cfg.LinkChecker.CFAccessClientID != "")
	} else {
		slog.Warn("link-live-checker NOT configured; resource 报告失效 falls back to legacy single-report-expires — set LINK_CHECKER_BASE_URL / LINK_CHECKER_API_KEY")
	}

	// Per-user contribution stats, derived from the registry's claim log and the
	// engine's merged proposals — the wiki stats endpoint they replace counted
	// rows in tables that are going away.
	galgameUserStatsSvc := galgameService.NewGalgameUserStatsService(catalogCli, gc)

	// Services
	authService := service.NewAuthService(userStateRepo, rdb, oauthClient, uc)
	userService := service.NewUserService(userStateRepo, userStatsRepo, rdb, gc, galgameUserStatsSvc, uc, communityCli)
	userContentService := service.NewUserContentService(userContentRepo, gc, galgameUserStatsSvc, uc, communityCli)
	messageSvc := msgService.NewMessageService(messageRepository, userStateRepo, uc)
	chatSvc := msgService.NewChatService(chatRepository, uc)
	notifier := msgService.NewNotifier(messageRepository)

	// Topic
	topicRepository := topicRepo.NewTopicRepository(db)
	topicListRepo := topicRepo.NewTopicListRepository(db)
	topicTaxonomyRepo := topicRepo.NewTopicTaxonomyRepository(db)
	replyRepository := topicRepo.NewReplyRepository(db)
	topicCommentRepo := topicRepo.NewCommentRepository(db)
	pollRepository := topicRepo.NewPollRepository(db)
	draftRepository := topicRepo.NewTopicDraftRepository(db)
	topicSvc := topicService.NewTopicService(topicRepository, topicListRepo, topicTaxonomyRepo, rdb, uc, userStateRepo)
	topicWriteSvc := topicService.NewTopicWriteService(topicRepository, topicTaxonomyRepo, replyRepository, userStateRepo, rdb, notifier, trustCheck, trustScan)
	replySvc := topicService.NewReplyService(replyRepository, topicCommentRepo, topicRepository, userStateRepo, uc, rdb, trustCheck, trustScan)
	commentSvc := topicService.NewCommentService(replyRepository, topicCommentRepo, userStateRepo, uc, rdb, trustCheck, trustScan)
	pollSvc := topicService.NewPollService(pollRepository, topicRepository, userStateRepo, uc, rdb, trustCheck, trustScan)
	draftSvc := topicService.NewDraftService(draftRepository)

	// Galgame
	// Community-backed comment BFF (charter step 03) — the unconditional galgame
	// comment backend on the `/comments` routes (router.go). The local repo owns
	// galgame_post_like + the legacy-id map (migration 057).
	galgameCommunityPostRepo := galgameRepo.NewCommunityPostRepository(db)
	galgameCommunityCommentSvc := galgameService.NewCommunityCommentService(communityCli, galgameCommunityPostRepo, uc, db)
	// Resource comment BFF (charter step 07) — the unconditional rating / website /
	// toolset comment backend on the `/comments` routes (router.go). Reuses the SAME
	// community client + local galgame_post_like repo (post-addressed likes are
	// region-agnostic).
	resourceCommentSvc := galgameService.NewResourceCommentService(communityCli, galgameCommunityPostRepo, uc, db)
	galgameResourceRepo := galgameRepo.NewResourceRepository(db)
	galgameResourceSvc := galgameService.NewResourceService(galgameResourceRepo, gc, uc, linkChecker, trustCheck, trustScan, cfg.Dlsite.LinkTemplate, cfg.Dlsite.CouponURL)
	galgameRatingRepo := galgameRepo.NewRatingRepository(db)
	galgameRatingSvc := galgameService.NewRatingService(galgameRatingRepo, gc, uc, trustCheck, trustScan)
	galgameQuizRepo := galgameRepo.NewQuizRepository(db)
	galgameQuizSvc := galgameService.NewQuizService(galgameQuizRepo, gc, uc, trustCheck, trustScan)
	creatorSvc := galgameService.NewCreatorService(galgameRatingRepo, galgameUserStatsSvc, uc)
	galgameLocalRepo := galgameRepo.NewGalgameRepository(db)
	galgameInteractionRepo := galgameRepo.NewGalgameInteractionRepository(db)
	galgameListRepo := galgameRepo.NewGalgameListRepository(db)
	galgameResourceMetaRepo := galgameRepo.NewGalgameResourceMetaRepository(db)
	galgameDetailRatingRepo := galgameRepo.NewGalgameDetailRatingRepository(db)
	galgameEnricher := galgameService.NewGalgameEnricher(galgameLocalRepo, galgameResourceMetaRepo, uc)
	// Core galgame service is built first: the entity (tag/official/engine)
	// detail services delegate their galgame list to it (shared local
	// filter/sort/hydrate), so they take it as a dependency.
	galgameCoreSvc := galgameService.NewGalgameService(
		galgameLocalRepo, galgameInteractionRepo, galgameListRepo,
		galgameResourceMetaRepo, galgameDetailRatingRepo, userStateRepo, gc, uc,
		cfg.Dlsite.LinkTemplate, cfg.Dlsite.CouponURL,
	)
	// Galgame collections (收藏夹): CRUD + membership. Delegates card hydration +
	// owner-name lookup to galgameCoreSvc so nothing is duplicated.
	galgameCollectionRepo := galgameRepo.NewGalgameCollectionRepository(db)
	galgameCollectionSvc := galgameService.NewCollectionService(galgameCollectionRepo, galgameCoreSvc, gc, uc, trustCheck, trustScan)
	galgameOfficialSvc := galgameService.NewOfficialService(gc, galgameCoreSvc)
	galgameEngineSvc := galgameService.NewEngineService(gc, galgameCoreSvc)
	galgameSeriesSvc := galgameService.NewSeriesService(gc, galgameCoreSvc)
	galgameTagSvc := galgameService.NewTagService(gc, galgameEnricher, galgameCoreSvc)
	galgameCalendarSvc := galgameService.NewCalendarService(gc, galgameEnricher)
	galgameDraftsSvc := galgameService.NewDraftsService(gc, galgameEnricher)
	galgameProxySvc := galgameService.NewGalgameProxyService(gc, galgameLocalRepo, uc)
	// Submission flow: submit / claim / patch-draft / delete-draft proxies
	// + local moemoepoint side effects. Per docs/galgame_wiki/07-submission.md.
	galgameSubmissionSvc := galgameService.NewSubmissionService(gc, catalogCli, galgameLocalRepo)
	// Submission moderation: the pending-claim queue + the four verdicts.
	galgameClaimReviewSvc := galgameService.NewClaimReviewService(gc, catalogCli)
	// Cron-driven ingestion of claim-state transitions: local stub lifecycle
	// plus the publication reward. Reads the registry's claim-event feed over
	// S2S — the wiki message feed it replaces retires with the wiki tables.
	galgameClaimSync := galgameService.NewGalgameClaimEventSync(catalogCli, galgameLocalRepo, rdb)
	// Mirrors editing-engine revisions into galgame_activity so the forum
	// activity timeline can show galgame edits (migrations 021 + 067). Reads the
	// catalog S2S client, not the wiki client: the engine is the author of every
	// galgame field edit, so its feed is the authoritative source (wave 156 N3).
	galgameRevisionSync := galgameService.NewGalgameEditRevisionSync(catalogCli, gc, db, rdb)

	// Website
	websiteRepository := websiteRepo.NewWebsiteRepository(db)
	websiteCategoryRepo := websiteRepo.NewCategoryRepository(db)
	websiteTagRepo := websiteRepo.NewTagRepository(db)
	websiteCoreSvc := websiteService.NewWebsiteService(
		websiteRepository, websiteCategoryRepo, websiteTagRepo, uc, communityCli, cfg.NextMoeAPI.ImageCDNBase,
	)
	websiteCategorySvc := websiteService.NewCategoryService(websiteCategoryRepo, websiteRepository, websiteTagRepo, cfg.NextMoeAPI.ImageCDNBase)
	websiteTagSvc := websiteService.NewTagService(websiteTagRepo, websiteRepository, websiteCategoryRepo, cfg.NextMoeAPI.ImageCDNBase)

	// Admin
	adminOverviewRepo := adminRepo.NewOverviewRepository(db)
	adminOverviewSvc := adminService.NewOverviewService(adminOverviewRepo)
	adminPurgeSvc := adminService.NewPurgeService(adminRepo.NewPurgeRepository(db), uc, communityCli)
	// Runtime permission overrides (permission-first authz, Phase 2 role layer +
	// Phase 3 user layer). PermissionOverrideSync owns the SINGLE Load path that
	// refreshes BOTH pkg/perm layers; boot, the 60s refresher, and every
	// write-through (role or user replace) go through it. The audit service serves
	// the append-only override change log (migration 064).
	adminRolePermRepo := adminRepo.NewRolePermissionRepository(db)
	adminUserPermRepo := adminRepo.NewUserPermissionRepository(db)
	adminPermSync := adminService.NewPermissionOverrideSync(adminRolePermRepo, adminUserPermRepo)
	adminRolePermSvc := adminService.NewRolePermissionService(adminRolePermRepo, adminPermSync)
	adminUserPermSvc := adminService.NewUserPermissionService(adminUserPermRepo, uc, adminPermSync)
	adminPermAuditSvc := adminService.NewPermissionAuditService(adminRepo.NewPermissionAuditRepository(db), uc)

	// Doc
	docArticleRepo := docRepo.NewArticleRepository(db)
	docCategoryRepo := docRepo.NewCategoryRepository(db)
	docTagRepo := docRepo.NewTagRepository(db)
	docArticleSvc := docService.NewArticleService(docArticleRepo, docCategoryRepo, cfg.NextMoeAPI.ImageCDNBase)
	docCategorySvc := docService.NewCategoryService(docCategoryRepo)
	docTagSvc := docService.NewTagService(docTagRepo)

	// Toolset
	toolsetRepository := toolsetRepo.NewToolsetRepository(db)
	toolsetResourceRepo := toolsetRepo.NewResourceRepository(db)
	toolsetPracticalityRepo := toolsetRepo.NewPracticalityRepository(db)
	toolsetPracticalitySvc := toolsetService.NewPracticalityService(toolsetPracticalityRepo)
	// Detail-page comment preview reads the community primitive (charter step 06a);
	// the full toolset comment area is served by the shared resource-comment BFF.
	toolsetCommentSvc := toolsetService.NewCommentService(uc, communityCli)
	toolsetResourceSvc := toolsetService.NewResourceService(toolsetResourceRepo, toolsetRepository, fileStorageClient, artCli, uc, trustCheck, trustScan)
	toolsetUploadSvc := toolsetService.NewUploadService(artCli, rdb, db)
	toolsetCoreSvc := toolsetService.NewToolsetService(
		toolsetRepository, toolsetResourceRepo, toolsetPracticalityRepo,
		fileStorageClient, uc, toolsetPracticalitySvc, toolsetCommentSvc,
		trustCheck, trustScan,
	)

	// Trust & Safety enforcement adapters — the "thin adapter" half of the
	// pipeline: each subject_kind wires hide/remove/author-lookup to existing
	// services/repos. galgame + user are ABSENT (human-only: galgame moderation
	// is galgame-side, user bans are IdP-side), so their callbacks no-op locally.
	//
	// galgame_comment is a legacy subject_kind: its rows migrated to community
	// posts (charter step 06a), so enforcement resolves the legacy id through the
	// map and tombstones the migrated post (hide == remove == tombstone; the
	// primitive has no S2S hide).
	galgameCommentEnforcer := galgameService.NewGalgameCommentEnforcer(communityCli, galgameCommunityPostRepo)
	trustRegistry := enforce.Registry{
		"forum_topic": {
			// No hard delete exists for topics → hide == remove (status=1).
			Hide: func(_ context.Context, id int) error {
				return topicRepository.UpdateFields(id, map[string]any{"status": 1})
			},
			Remove: func(_ context.Context, id int) error {
				return topicRepository.UpdateFields(id, map[string]any{"status": 1})
			},
			AuthorID: func(_ context.Context, id int) (int, error) {
				t, err := topicRepository.FindByID(id)
				if err != nil {
					return 0, nil
				}
				return t.UserID, nil
			},
		},
		"forum_reply": {
			Hide:   func(_ context.Context, id int) error { return replyRepository.SetStatus(id, 1) },
			Remove: func(_ context.Context, id int) error { return replySvc.ModerationRemove(id) },
			AuthorID: func(_ context.Context, id int) (int, error) {
				r, err := replyRepository.FindByID(id)
				if err != nil {
					return 0, nil
				}
				return r.UserID, nil
			},
		},
		"forum_comment": {
			Hide:   func(_ context.Context, id int) error { return topicCommentRepo.SetStatus(id, 1) },
			Remove: func(_ context.Context, id int) error { return commentSvc.ModerationRemove(id) },
			AuthorID: func(_ context.Context, id int) (int, error) {
				c, err := topicCommentRepo.FindCommentByID(id)
				if err != nil {
					return 0, nil
				}
				return c.UserID, nil
			},
		},
		"galgame_comment": {
			// hide == remove == tombstone the migrated community post (via the map).
			Hide:     galgameCommentEnforcer.Tombstone,
			Remove:   galgameCommentEnforcer.Tombstone,
			AuthorID: galgameCommentEnforcer.AuthorID,
		},
	}
	// warn_user is record-only for now (no system-sender user for a targeted
	// notice) — pass nil; the dispatcher no-ops warn gracefully.
	trustEnforce := enforce.NewService(db, trustRegistry, nil)

	// Handlers
	app := &App{
		DB: db, Redis: rdb, Mailer: mailer, Config: cfg, OAuthClient: oauthClient,
		UserState:                      userStateRepo,
		UserClient:                     uc,
		OAuthHandler:                   handler.NewOAuthHandler(authService, cfg.Server.Mode == "prod", communityBooster),
		UserHandler:                    handler.NewUserHandler(userService, userContentService),
		UserProfileHandler:             handler.NewProfileHandler(oauthClient, uc),
		HomeHandler:                    homeHandler.NewHomeHandler(homeService.NewHomeService(homeRepo.NewHomeRepository(db), gc, uc, rdb)),
		TopicHandler:                   topicHandler.NewTopicHandler(topicSvc, topicWriteSvc),
		TopicDraftHandler:              topicHandler.NewTopicDraftHandler(draftSvc),
		ReplyHandler:                   topicHandler.NewReplyHandler(replySvc),
		TopicCommentHandler:            topicHandler.NewCommentHandler(commentSvc),
		PollHandler:                    topicHandler.NewPollHandler(pollSvc),
		MessageHandler:                 msgHandler.NewMessageHandler(messageSvc),
		MessageChatHandler:             msgHandler.NewChatHandler(chatSvc),
		AdminOverviewHandler:           adminHandler.NewOverviewHandler(adminOverviewSvc),
		AdminPurgeHandler:              adminHandler.NewPurgeHandler(adminPurgeSvc),
		AdminRolePermissionHandler:     adminHandler.NewRolePermissionHandler(adminRolePermSvc),
		AdminUserPermissionHandler:     adminHandler.NewUserPermissionHandler(adminUserPermSvc),
		AdminPermissionAuditHandler:    adminHandler.NewPermissionAuditHandler(adminPermAuditSvc),
		RankingHandler:                 rankingHandler.NewRankingHandler(rankingService.NewRankingService(rankingRepo.NewRankingRepository(db), gc, uc)),
		SectionHandler:                 sectionHandler.NewSectionHandler(sectionService.NewSectionService(sectionRepo.NewSectionRepository(db), uc)),
		DocArticleHandler:              docHandler.NewArticleHandler(docArticleSvc),
		DocCategoryHandler:             docHandler.NewCategoryHandler(docCategorySvc),
		DocTagHandler:                  docHandler.NewTagHandler(docTagSvc),
		WebsiteHandler:                 websiteHandler.NewWebsiteHandler(websiteCoreSvc),
		WebsiteCategoryHandler:         websiteHandler.NewCategoryHandler(websiteCategorySvc),
		WebsiteTagHandler:              websiteHandler.NewTagHandler(websiteTagSvc),
		UpdateHandler:                  updateHandler.NewUpdateHandler(updateRepo.NewUpdateRepository(db)),
		FriendLinkHandler:              friendHandler.NewFriendLinkHandler(friendRepo.NewFriendLinkRepository(db), cfg.NextMoeAPI.ImageCDNBase),
		TrustHandler:                   trustHandler.NewTrustHandler(trustService.NewTrustService(trustCli, cfg.Trust.Site), trustEnforce, cfg.Trust.CallbackSecret),
		RSSHandler:                     rssHandler.NewRSSHandler(rssRepo.NewRSSRepository(db), gc, uc),
		GalgameHandler:                 galgameHandler.NewGalgameHandler(galgameCoreSvc),
		GalgameCollectionHandler:       galgameHandler.NewGalgameCollectionHandler(galgameCollectionSvc),
		GalgameCommunityCommentHandler: galgameHandler.NewCommunityCommentHandler(galgameCommunityCommentSvc),
		ResourceCommentHandler:         galgameHandler.NewResourceCommentHandler(resourceCommentSvc),
		GalgameResourceHandler:         galgameHandler.NewResourceHandler(galgameResourceSvc),
		GalgameRatingHandler:           galgameHandler.NewRatingHandler(galgameRatingSvc),
		GalgameQuizHandler:             galgameHandler.NewQuizHandler(galgameQuizSvc),
		CreatorHandler:                 galgameHandler.NewCreatorHandler(creatorSvc),
		GalgameEntityHandler: galgameHandler.NewEntityHandler(
			galgameOfficialSvc, galgameEngineSvc, galgameSeriesSvc, galgameTagSvc,
		),
		GalgameCalendarHandler:      galgameHandler.NewCalendarHandler(galgameCalendarSvc),
		GalgameDraftsHandler:        galgameHandler.NewDraftsHandler(galgameDraftsSvc),
		GalgameProxyHandler:         galgameHandler.NewGalgameProxyHandler(galgameProxySvc),
		GalgameSubmissionHandler:    galgameHandler.NewSubmissionHandler(galgameSubmissionSvc),
		GalgameClaimReviewHandler:   galgameHandler.NewClaimReviewHandler(galgameClaimReviewSvc),
		GalgameEditHandler:          galgameHandler.NewEditHandler(catalogCli, gc, uc, notifier, galgameLocalRepo),
		ActivityHandler:             activityHandler.NewActivityHandler(activityService.NewActivityService(activityRepo.NewActivityRepository(db), gc, uc, rdb)),
		ImageHandler:                imageHandler.NewImageHandler(imageService.NewImageService(imageRepo.NewImageRepository(db), imgCli, catalogCli)),
		SearchHandler:               searchHandler.NewSearchHandler(searchService.NewSearchService(searchRepo.NewSearchRepository(db), gc, galgameEnricher, uc)),
		ToolsetHandler:              toolsetHandler.NewToolsetHandler(toolsetCoreSvc),
		ToolsetPracticalityHandler:  toolsetHandler.NewPracticalityHandler(toolsetPracticalitySvc),
		ToolsetResourceHandler:      toolsetHandler.NewResourceHandler(toolsetResourceSvc),
		ToolsetUploadHandler:        toolsetHandler.NewUploadHandler(toolsetUploadSvc),
		CronStop:                    cronPkg.Start(db, rdb, imgCli, galgameClaimSync.Run, galgameRevisionSync.Run),
	}

	// Load the runtime permission overrides (BOTH the role and user layers) into
	// pkg/perm and keep them fresh. A boot-load failure warns and leaves the
	// compiled baseline in place — the override tables being unreachable must never
	// block startup or degrade requests (the compiled baseline is the safe known
	// state).
	if err := adminPermSync.Load(context.Background()); err != nil {
		slog.Warn("加载权限覆盖失败, 暂时沿用编译期基线", "error", err)
	}
	app.RolePermStop = adminPermSync.StartRefresher(60 * time.Second)

	// Fiber
	//
	// ReadBufferSize bumped to 16KB. Fiber's default is 4KB which is too
	// tight for our SSR request flow: the Nuxt server forwards the
	// authenticated user's cookies to /api, and once
	// pinia-plugin-persistedstate has spread several stores (user,
	// settings, sidebar, etc.) plus the OAuth session cookie across a
	// long-lived session, the Cookie header alone can creep past 4KB and
	// surface as the rather uninformative
	// "Request Header Fields Too Large" (which silently empties pages
	// because the SSR fetch fails). The frontend kunFetch only forwards
	// the session cookie now to keep things tight, but the bump here is
	// defense in depth for real-world browsers that accumulate
	// third-party cookies.
	fiberApp := fiber.New(fiber.Config{
		ErrorHandler:   globalErrorHandler,
		BodyLimit:      10 * 1024 * 1024,
		ReadBufferSize: 16 * 1024,
	})
	fiberApp.Use(recover.New())
	app.Fiber = fiberApp

	app.setupRoutes()
	return app
}

func globalErrorHandler(c fiber.Ctx, err error) error {
	if appErr, ok := err.(*errors.AppError); ok {
		return response.Error(c, appErr)
	}
	slog.Error("未处理的错误", "error", err.Error(), "path", c.Path(), "method", c.Method())
	return response.Error(c, errors.ErrInternal("服务器内部错误"))
}
