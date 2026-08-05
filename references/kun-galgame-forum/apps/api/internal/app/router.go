package app

import (
	"kun-galgame-api/internal/middleware"
	"kun-galgame-api/pkg/perm"

	"github.com/gofiber/fiber/v3"
	fiberCors "github.com/gofiber/fiber/v3/middleware/cors"
)

func (a *App) setupRoutes() {
	// Session cookies are Secure (HTTPS-only) in prod, plain in dev over HTTP.
	// Drives renewSlidingSession's re-issued cookie; mirrors the login
	// handler's secure flag (NewOAuthHandler(_, cfg.Server.Mode == "prod")).
	middleware.SecureCookies = a.Config.Server.Mode == "prod"

	a.Fiber.Use(fiberCors.New(middleware.CORS(a.Config.CORS.AllowOrigins)))

	// Liveness probe for the container HEALTHCHECK (`server healthcheck`) and
	// compose depends_on gates. Plain 200 — deliberately does NOT touch DB/Redis
	// so a transient backing-store blip can't flap the container as unhealthy.
	a.Fiber.Get("/healthz", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	api := a.Fiber.Group("/api")

	// ════════════════════════════════════════════
	// PUBLIC routes (no auth required)
	// Must be registered BEFORE authed group to avoid
	// Fiber Group("") middleware intercepting them.
	// ════════════════════════════════════════════

	api.Get("/home", a.HomeHandler.GetHome)

	// Trust & Safety enforcement callback (PUBLIC — authenticated by the HMAC
	// X-Trust-Signature over the raw body, not a session). Idempotent.
	api.Post("/trust/callback", a.TrustHandler.Callback)

	// Auth (public). Identity changes (password / email / username / bio /
	// avatar) all live in the OAuth admin UI now — kungal owns nothing
	// that needs an /auth/* email-code flow.
	auth := api.Group("/auth")
	auth.Post("/oauth/callback", a.OAuthHandler.Callback)
	auth.Post("/logout", a.OAuthHandler.Logout)

	// User (authenticated, fixed paths — registered before :id to avoid conflicts).
	// Bio / username / email / ban / delete were here pre-OAuth; all moved to OAuth.
	userAuth := middleware.Auth(a.Redis, a.OAuthClient)
	// No rate limiter: the once-per-day gate is the `daily_check_in` flag
	// (reset at calendar midnight by the daily cron) enforced atomically in
	// StateRepository.CheckIn. A 24h-rolling rate limiter here used to block
	// legitimate next-day check-ins (its window spilled past midnight) and
	// masked the real "已签到" message with a generic "操作过于频繁" 400.
	api.Post("/user/check-in", userAuth, a.UserHandler.CheckIn)
	api.Get("/user/status", userAuth, a.UserHandler.GetStatus)
	// Notification preferences (own only) — the muted-category opt-out set that
	// suppresses the red dot / badges. Fixed path, MUST stay before /user/:id.
	api.Get("/user/notification-preferences", userAuth, a.UserHandler.GetNotificationPreferences)
	api.Put("/user/notification-preferences", userAuth, a.UserHandler.UpdateNotificationPreferences)
	// Unified moemoepoint ledger (own only) — fixed path, before /user/:id.
	api.Get("/user/moemoepoint/log", userAuth, a.UserHandler.GetMoemoepointLog)
	// @mention autocomplete — fixed path, MUST stay before /user/:id (else
	// "search" is captured as :id). Proxies OAuth /users/search (not cached).
	api.Get("/user/search", userAuth, a.UserHandler.SearchMention)

	// Creator-role application: forum checks its eligibility (galgame PR/galgame
	// stats + own 简评), then files on the central OAuth queue. Role grant +
	// admin review live in OAuth (contract owned there, not yet mirrored here).
	api.Get("/user/creator/status", userAuth, a.CreatorHandler.Status)
	api.Post("/user/creator/apply", userAuth, a.CreatorHandler.Apply)

	// Self-edit endpoints — proxy to OAuth /auth/me family, the session-
	// stored bearer is attached inside each handler. See
	// docs/oauth/02-user-profile.md.
	api.Put("/user/bio", userAuth, a.UserProfileHandler.UpdateBio)
	api.Put("/user/username", userAuth, a.UserProfileHandler.UpdateUsername)
	api.Post("/user/avatar", userAuth, a.UserProfileHandler.UploadAvatar)

	// User (public, parameterized — AFTER fixed paths)
	api.Get("/user/:id/floating", a.UserHandler.GetFloatingCard)
	api.Get("/user/:id", a.UserHandler.GetProfile)
	api.Get("/user/:id/galgames", a.UserHandler.GetUserGalgames)
	api.Get("/user/:id/galgame-comments", a.UserHandler.GetUserGalgameComments)
	api.Get("/user/:id/topics", a.UserHandler.GetUserTopics)
	api.Get("/user/:id/replies", a.UserHandler.GetUserReplies)
	api.Get("/user/:id/comments", a.UserHandler.GetUserComments)
	api.Get("/user/:id/resources", a.UserHandler.GetUserResources)
	api.Get("/user/:id/ratings", a.UserHandler.GetUserRatings)
	api.Get("/user/:id/toolsets", a.ToolsetHandler.GetUserToolsets)

	// Ranking (public)
	api.Get("/ranking/galgame", a.RankingHandler.GetGalgameRanking)
	api.Get("/ranking/topic", a.RankingHandler.GetTopicRanking)
	api.Get("/ranking/user", a.RankingHandler.GetUserRanking)

	// Section & Category (public)
	api.Get("/section", a.SectionHandler.GetSectionTopics)
	api.Get("/category", a.SectionHandler.GetCategories)

	// Doc (public reads)
	api.Get("/doc/article", a.DocArticleHandler.GetArticles)
	api.Get("/doc/article/:slug", a.DocArticleHandler.GetArticleBySlug)
	api.Get("/doc/category", a.DocCategoryHandler.GetCategories)
	api.Get("/doc/tag", a.DocTagHandler.GetTags)

	// Website (public reads)
	api.Get("/website-category/:name", a.WebsiteCategoryHandler.GetWebsiteCategory)
	api.Get("/website-tag", a.WebsiteTagHandler.GetWebsiteTags)
	api.Get("/website-tag/:name", a.WebsiteTagHandler.GetWebsiteTagDetail)

	// Update (public reads)
	api.Get("/update/history", a.UpdateHandler.GetHistory)
	api.Get("/update/todo", a.UpdateHandler.GetTodos)

	// Friend links (public read — rendered on /friend-links)
	api.Get("/friend-link", a.FriendLinkHandler.List)

	// Effective role→permission bundles (PUBLIC — non-sensitive config that only
	// tells the FE which capabilities a role holds so it can show/hide affordances;
	// it grants nothing).
	api.Get("/perm/bundles", a.AdminRolePermissionHandler.GetBundles)

	// Activity (public)
	api.Get("/activity", a.ActivityHandler.GetActivity)
	api.Get("/activity/tab", a.ActivityHandler.GetTab)
	api.Get("/activity/timeline", a.ActivityHandler.GetTimeline)

	// Galgame rating — moved to optAuth group below so service handlers
	// receive a non-zero `currentUserID` for logged-in callers and can
	// hydrate the per-row `isLiked` / liked-by-me state. Leaving them
	// here in the `api` group made optionalUID return 0 unconditionally,
	// which silently broke the K-PR `FindLikedSet` batch fix.

	// Resource topics (public, same as topic but filtered to resource sections)
	api.Get("/resource", a.TopicHandler.GetResourceList)

	// Search (public)
	api.Get("/search", a.SearchHandler.Search)

	// RSS (public)
	api.Get("/rss/topic", a.RSSHandler.GetTopicRSS)
	api.Get("/rss/galgame", a.RSSHandler.GetGalgameRSS)

	// Toolset resource detail (public)
	api.Get("/toolset/:id/resource/detail", a.ToolsetResourceHandler.GetResourceDetail)

	// Galgame galgame proxies (public reads)
	api.Get("/galgame", a.GalgameHandler.GetList)
	// `/galgame/check` (VNDB-ID precheck) was retired when the publish
	// wizard moved to name-based search — no FE caller remains. Removed
	// from registration; re-add only if a downstream brings the check
	// flow back.
	// /galgame/mine and /galgame/search/wizard MUST be registered BEFORE
	// /galgame/:gid below — Fiber matches by registration order and a
	// catch-all `:gid` happily binds to the literal "mine" / "search",
	// which would route to GetDetail and then fail with Atoi("mine").
	// Both endpoints require auth; inline userAuth here so we don't have
	// to predeclare the `authed` group above the optAuth section.
	api.Get("/galgame/mine", userAuth, a.GalgameSubmissionHandler.ListMine)
	api.Get(
		"/galgame/search/wizard",
		userAuth,
		a.GalgameSubmissionHandler.SearchWithPending,
	)
	// Lightweight galgame search shared by the associate-galgame pickers (出题
	// modal + series editor) via GalgameSearchAutocomplete. Public galgame
	// Meilisearch search; the handler lives on the quiz handler for historical
	// reasons. Literal 3-segment path, registered before /galgame/:gid.
	api.Get("/galgame/search/picker", a.GalgameQuizHandler.SearchGalgames)
	// Galgame 发售月历 — proxies galgame /galgame/calendar(+pending/tba), enriched
	// with forum-local card data. Public + SFW-default. Same registration-order
	// rule as /galgame/mine above: the literal "calendar" segment must come
	// before the /galgame/:gid catch-all or it'd bind as gid="calendar".
	api.Get("/galgame/calendar", a.GalgameCalendarHandler.GetMonth)
	api.Get("/galgame/calendar/pending", a.GalgameCalendarHandler.GetPending)
	api.Get("/galgame/calendar/tba", a.GalgameCalendarHandler.GetTBA)
	api.Get("/galgame/calendar/upcoming", a.GalgameCalendarHandler.GetUpcoming)
	// Unclaimed VNDB drafts (status=2) — powers the detail page's "未发布的游戏"
	// modal. Public + SFW-default, paginated. Literal "drafts" segment, so it
	// must precede the /galgame/:gid catch-all — same rule as /galgame/calendar.
	api.Get("/galgame/drafts", a.GalgameDraftsHandler.GetDrafts)
	// Editing-engine reads (E3a; public like the galgame revision history always
	// was): the engine-backed history diff + per-game proposal list. The
	// old-wire read proxies (/revisions*, /prs*, /links, /aliases) retired in
	// E3b — every kungal consumer reads the engine now. /edit/revisions lives
	// in the optAuth group below: a logged-in reviewer gets can_revert.
	api.Get("/galgame/:gid/edit/diff", a.GalgameEditHandler.Diff)
	// Per-game proposal list (E3b; public like the old wire's PR list) —
	// the owner's per-game review surface and everyone's transparency read.
	api.Get("/galgame/:gid/edit/proposals", a.GalgameEditHandler.GameProposals)
	// `/galgame/:gid/contributors` is unused — the FE contributor view
	// reads from detail's embedded `contributor[]` array now
	// (apps/web/.../components/galgame/contributor/Container.vue notes
	// the removal). Drop the proxy route to keep the public surface
	// honest; re-add if a downstream rebuilds the standalone view.
	// NOTE: galgame detail sub-routes (/pr/all, /link/all, etc.) are
	// registered in the optAuth group below to avoid Fiber route shadowing
	// by /galgame/:gid.
	api.Get("/galgame-tag", a.GalgameEntityHandler.GetTagList)
	api.Get("/galgame-tag/search", a.GalgameEntityHandler.SearchTags)
	api.Get("/galgame-tag/multi", a.GalgameEntityHandler.GetMultiTagGalgames)
	// Taxonomy detail :id is a CATALOG id (doc 106 R1) — the browse pages link
	// into the /galgame/{tag,official,engine}/ URL space (doc 146) and every
	// retired frontend URL, wiki-id or `/c/`-segmented, is an FE-side redirect
	// shell that never reaches this API.
	//
	// NOTE the two namespaces are independent: this is the API's OWN kebab-case
	// endpoint space and it deliberately did NOT move with the frontend rename.
	// Moving it would drop these under the /galgame/:gid param route, making
	// them order-dependent against it — the same shadowing hazard the detail
	// sub-routes above are already grouped to dodge — for no gain: no client
	// addresses the API by the frontend's URL shape.
	api.Get("/galgame-tag/:id", a.GalgameEntityHandler.GetTagDetail)
	api.Get("/galgame-official", a.GalgameEntityHandler.GetOfficialList)
	api.Get("/galgame-official/search", a.GalgameEntityHandler.SearchOfficials)
	// Legacy wiki-id → catalog-id resolver behind the /galgame-official/{wikiId}
	// redirect shell. Must precede the :id catch-all.
	api.Get("/galgame-official/legacy/:id", a.GalgameEntityHandler.ResolveLegacyOfficial)
	api.Get("/galgame-official/:id", a.GalgameEntityHandler.GetOfficialDetail)
	api.Get("/galgame-engine", a.GalgameEntityHandler.GetEngineList)
	api.Get("/galgame-engine/:id", a.GalgameEntityHandler.GetEngineDetail)
	api.Get("/galgame-series", a.GalgameEntityHandler.GetSeriesList)
	// Rich index/panel cards. Must precede the :id catch-all.
	api.Get("/galgame-series/cards", a.GalgameEntityHandler.GetSeriesCards)
	api.Get("/galgame-series/:id", a.GalgameEntityHandler.GetSeriesDetail)
	// The /galgame-series family retired with the wiki series vocabulary (P3):
	// 146 wiki series, only 6 of which correspond to anything in the catalog.
	// `/galgame-resource` list — moved to optAuth below (FE list cards
	// show the heart icon and need the viewer's per-row like state).
	// `/galgame-rating/all` stays here in the public group: the rating
	// Card.vue doesn't render a like toggle, so optAuth would just be
	// dead-weight middleware on a high-traffic list endpoint.
	api.Get("/galgame-rating/all", a.GalgameRatingHandler.GetAllRatings)
	// Answerer records for the 查看详情 panel. OptionalAuth: a viewer who has
	// answered (or authored) this quiz also gets each answerer's submitted answer;
	// a non-answerer gets who answered + grade only (submitted leaks the answer).
	// 3-segment, so it never collides with the 2-segment /galgame-quiz/:id.
	api.Get(
		"/galgame-quiz/:id/answers",
		middleware.OptionalAuth(a.Redis, a.OAuthClient),
		a.GalgameQuizHandler.GetQuizAnswers,
	)

	// ════════════════════════════════════════════
	// OPTIONAL AUTH routes (public but attach user if logged in)
	// ════════════════════════════════════════════

	optAuth := api.Group("", middleware.OptionalAuth(a.Redis, a.OAuthClient))
	// Galgame resource list + detail family — all need optionalUID for
	// the per-row `isLiked` flag (batch query via FindLikedSet on lists,
	// single query on detail).
	optAuth.Get("/galgame-resource", a.GalgameResourceHandler.GetResourceList)
	optAuth.Get("/galgame-resource/:id/detail", a.GalgameResourceHandler.GetResourceDownloadDetail)
	optAuth.Get("/galgame-resource/:id", a.GalgameResourceHandler.GetResourceDetail)

	// Galgame rating detail — currentUserID feeds containsInt against
	// likerIDs to populate the per-viewer `isLiked` flag the Like.vue
	// component renders.
	optAuth.Get("/galgame-rating/:id", a.GalgameRatingHandler.GetRatingDetail)

	// Galgame quiz play/detail — optional auth so a logged-in viewer who has
	// already answered gets the revealed answer key + their result in
	// `my_answer`. (/galgame-quiz/mine/answered is 3-segment, so it never
	// collides with this 2-segment :id route.)
	//
	// `/galgame-quiz/all` is optional-auth too — a logged-in viewer's list cards
	// carry their own status (答对/答错/未答). Registered BEFORE `:id` so the
	// static path wins over the param route.
	optAuth.Get("/galgame-quiz/all", a.GalgameQuizHandler.GetAllQuizzes)
	optAuth.Get("/galgame-quiz/:id", a.GalgameQuizHandler.GetQuizPlay)

	// Topic (optional auth for interaction status)
	// Private per-user topic drafts. Registered here (on `api`, with an explicit
	// auth middleware) BEFORE the /topic/:tid routes below because Fiber matches
	// in REGISTRATION ORDER: a later static /topic/draft would otherwise be
	// captured by the earlier /topic/:tid param route (tid="draft" → 400).
	topicDraftAuth := middleware.Auth(a.Redis, a.OAuthClient)
	api.Get("/topic/draft", topicDraftAuth, a.TopicDraftHandler.List)
	api.Post("/topic/draft", topicDraftAuth, a.TopicDraftHandler.Save)
	api.Get("/topic/draft/:id", topicDraftAuth, a.TopicDraftHandler.Get)
	api.Delete("/topic/draft/:id", topicDraftAuth, a.TopicDraftHandler.Delete)

	optAuth.Get("/topic", a.TopicHandler.GetList)
	optAuth.Get("/topic/:tid", a.TopicHandler.GetDetail)
	optAuth.Get("/topic/:tid/upvotes", a.TopicHandler.GetUpvotes)
	optAuth.Get("/topic/:tid/reaction/history", a.TopicHandler.GetTopicReactionHistory)
	optAuth.Get("/topic/:tid/reply", a.ReplyHandler.GetReplies)
	optAuth.Get("/topic/:tid/reply/detail", a.ReplyHandler.GetReplyDetail)
	optAuth.Get("/topic/:tid/reply/locate", a.ReplyHandler.GetReplyLocate)
	optAuth.Get("/topic/:tid/poll/topic", a.PollHandler.GetPollsByTopic)
	optAuth.Get("/topic/:tid/poll/log", a.PollHandler.GetVoteLog)

	// Galgame detail sub-routes (MUST be before /:gid to avoid shadowing)
	optAuth.Get("/galgame/:gid/resource/all", a.GalgameResourceHandler.GetGalgameResources)
	// Community-backed comment READS (anonymous-readable). These MUST be mounted
	// BEFORE the mandatory-auth boundary below — Fiber middleware is stack-ordered,
	// so anything registered after it is login-gated regardless of the group
	// handle. The write half mounts after the boundary. Community is now the
	// unconditional comment backend (the legacy /comment reads were retired in
	// charter step 06a); an unconfigured client degrades reads to empty pages.
	a.GalgameCommunityCommentHandler.RegisterReads(optAuth)
	// Community-backed resource comment READS (rating / website / toolset /
	// galgame-resource / quiz), anonymous-readable. Same stack-position rule as
	// above: mounted BEFORE the mandatory-auth boundary so anonymous reads reach
	// the handler. The galgame-resource / quiz comment paths are 3-segment, so they
	// never collide with the 2-segment /galgame-resource/:id and /galgame-quiz/:id
	// detail reads registered earlier (a Fiber `:param` never spans a `/`). The
	// quiz area additionally gates itself server-side for a viewer who has not
	// answered a spoiler-bearing quiz — see service/resource_comment_gate.go.
	a.ResourceCommentHandler.RegisterReads(optAuth)
	optAuth.Get("/galgame/:gid/link/all", a.GalgameProxyHandler.GetGalgameLinks)
	// Engine-backed revision history (E3a/E3b): public read, but a logged-in
	// reviewer (moderator / the game's creator) additionally gets can_revert.
	optAuth.Get("/galgame/:gid/edit/revisions", a.GalgameEditHandler.Revisions)
	optAuth.Get("/galgame/:gid", a.GalgameHandler.GetDetail)

	// Galgame collections (收藏夹) — public/restricted visibility resolved with
	// the optional viewer id. /galgame/collection/:cid is 3-segment so it never
	// collides with the 2-segment /galgame/:gid above.
	optAuth.Get("/galgame/collection/:cid", a.GalgameCollectionHandler.GetDetail)
	optAuth.Get("/user/:id/collections", a.GalgameCollectionHandler.GetUserCollections)

	// Website (optional auth for like/favorite status)
	optAuth.Get("/website", a.WebsiteHandler.GetWebsites)
	optAuth.Get("/website/:domain", a.WebsiteHandler.GetWebsiteDetail)

	// Toolset (optional auth for practicality "mine" field)
	optAuth.Get("/toolset", a.ToolsetHandler.GetList)
	optAuth.Get("/toolset/:id", a.ToolsetHandler.GetDetail)
	optAuth.Get("/toolset/:id/practicality", a.ToolsetPracticalityHandler.GetPracticality)

	// ════════════════════════════════════════════
	// AUTHENTICATED routes (require valid session)
	// ════════════════════════════════════════════

	authed := api.Group("", middleware.Auth(a.Redis, a.OAuthClient))
	authed.Get("/auth/me", a.OAuthHandler.Me)

	// The current user's own effective permissions (role-derived set + personal
	// deltas). Feeds FE visibility only — it grants nothing.
	authed.Get("/perm/mine", a.AdminUserPermissionHandler.GetMine)

	// Topic (authenticated)
	// Static /topic/interactions/mine wins over /topic/:tid (static segments beat
	// the param route), so feed cards hydrate the viewer's own 收藏 + reactions.
	authed.Get("/topic/interactions/mine", a.TopicHandler.MyInteractions)
	authed.Post("/topic", a.TopicHandler.Create)
	authed.Put("/topic/:tid", a.TopicHandler.Update)
	authed.Put("/topic/:tid/like", a.TopicHandler.ToggleLike)
	authed.Put("/topic/:tid/dislike", a.TopicHandler.ToggleDislike)
	authed.Put("/topic/:tid/upvote", a.TopicHandler.Upvote)
	authed.Put("/topic/:tid/favorite", a.TopicHandler.ToggleFavorite)
	authed.Put("/topic/:tid/reaction", a.TopicHandler.ToggleReaction)
	authed.Put("/topic/:tid/hide", a.TopicHandler.ToggleHide)
	authed.Put("/topic/:tid/best-answer", a.TopicHandler.SetBestAnswer)

	// Reply (authenticated)
	authed.Post("/topic/:tid/reply", a.ReplyHandler.CreateReply)
	authed.Put("/topic/:tid/reply", a.ReplyHandler.UpdateReply)
	authed.Delete("/topic/:tid/reply", a.ReplyHandler.DeleteReply)
	authed.Put("/topic/:tid/reply/like", a.ReplyHandler.ToggleReplyLike)
	authed.Put("/topic/:tid/reply/dislike", a.ReplyHandler.ToggleReplyDislike)
	authed.Put("/topic/:tid/reply/reaction", a.ReplyHandler.ToggleReplyReaction)
	authed.Put("/topic/:tid/reply/pin", a.ReplyHandler.PinReply)

	// Comment (authenticated)
	authed.Post("/topic/:tid/comment", a.TopicCommentHandler.CreateComment)
	authed.Put("/topic/:tid/comment", a.TopicCommentHandler.UpdateComment)
	authed.Put("/topic/:tid/comment/like", a.TopicCommentHandler.ToggleCommentLike)
	authed.Delete("/topic/:tid/comment", a.TopicCommentHandler.DeleteComment)

	// Poll (authenticated)
	authed.Post("/topic/:tid/poll", a.PollHandler.CreatePoll)
	authed.Put("/topic/:tid/poll", a.PollHandler.UpdatePoll)
	authed.Delete("/topic/:tid/poll", a.PollHandler.DeletePoll)
	authed.Post("/topic/:tid/poll/vote", a.PollHandler.Vote)

	// Message (authenticated)
	authed.Get("/message", a.MessageHandler.GetMessages)
	authed.Get("/message/muted", a.MessageHandler.GetMutedMessages)
	authed.Delete("/message/:id", a.MessageHandler.DeleteMessage)
	authed.Put("/message/system/read", a.MessageHandler.MarkAllRead)
	authed.Get("/message/admin", a.MessageHandler.GetSystemMessages)
	authed.Put("/message/admin/read", a.MessageHandler.MarkAdminRead)
	authed.Get("/message/nav/system", a.MessageHandler.GetNavSummary)
	authed.Get("/message/nav/contact", a.MessageChatHandler.GetNavContact)
	authed.Get("/message/chat/history", a.MessageChatHandler.GetChatHistory)
	authed.Post("/message/chat/send", a.MessageChatHandler.SendChatMessage)
	authed.Post("/message/chat/recall", a.MessageChatHandler.RecallChatMessage)

	// Image upload (authenticated)
	authed.Post("/image/topic", a.ImageHandler.UploadTopicImage)
	// Cover / banner / icon upload (doc, friend-link, website) — uploads under
	// the `topic` preset and returns {hash, url, ...}; the caller stores the
	// hash on its entity's *_image_hash column.
	authed.Post("/image/cover", a.ImageHandler.UploadCoverImage)
	// Chat / private-message inline image upload (own `message` preset).
	authed.Post("/image/message", a.ImageHandler.UploadMessageImage)
	// U2 (K-PR3a): galgame cover / screenshot upload — proxies a
	// single image to image_service under one of the gated presets and
	// returns the resulting {hash, url, ...} to the FE so it can attach
	// the hash to a covers[] or screenshots[] row on the next PUT/PR.
	authed.Post("/image/galgame", a.ImageHandler.UploadGalgameImage)

	// Content reporting → infra Trust & Safety (Phase 1). Generic passthrough;
	// the reporter is the session user, subject kind/id come from the body.
	authed.Get("/report/reasons", a.TrustHandler.GetReasons)
	authed.Post("/report/submit", a.TrustHandler.SubmitReport)

	// Website interactions (authenticated)
	authed.Put("/website/:domain/like", a.WebsiteHandler.ToggleLike)
	authed.Put("/website/:domain/favorite", a.WebsiteHandler.ToggleFavorite)

	// Galgame submission flow (authenticated, any role). Each of these is one
	// semantic action on the registry's claim; there is no "patch my draft"
	// any more, because a draft's FIELDS are edited through the same editing
	// face as any other entry's and only its STATE moves here.
	//
	// DELETE is 撤回, not deletion: the claim returns to draft and the registry
	// row — an identity other products and the edit history point at — survives.
	//
	// /mine + /search/wizard are registered earlier (above the
	// /galgame/:gid catch-all) because Fiber matches in registration
	// order; see the comment near api.Get("/galgame/mine", ...) above.
	authed.Post("/galgame/submit", a.GalgameSubmissionHandler.Submit)
	authed.Post("/galgame/:gid/claim", a.GalgameSubmissionHandler.Claim)
	authed.Post("/galgame/:gid/resubmit", a.GalgameSubmissionHandler.Resubmit)
	authed.Delete("/galgame/:gid", a.GalgameSubmissionHandler.Withdraw)

	// The wiki message stream (/galgame/messages/mine + the read-state marker)
	// retired in wave 169: the upstream feed 404s since wave-161 P5, and the
	// claim_event feed + forum-local messages replaced it (N waves). The
	// read-state table stays in the DB as history; nothing reads it.

	// Galgame interactions (authenticated, local). The static
	// /galgame/interactions/mine is declared before the :gid routes
	// (static segments win over :gid).
	authed.Get("/galgame/interactions/mine", a.GalgameHandler.MyInteractions)
	authed.Put("/galgame/:gid/like", a.GalgameHandler.ToggleLike)
	// Galgame collections (收藏夹). Favorite is now membership in one or more
	// collections. /galgame/collection is static and 2-segment; the picker
	// read/write hang off the /galgame/:gid/collections param path.
	authed.Post("/galgame/collection", a.GalgameCollectionHandler.Create)
	authed.Patch("/galgame/collection/:cid", a.GalgameCollectionHandler.Update)
	authed.Delete("/galgame/collection/:cid", a.GalgameCollectionHandler.Delete)
	authed.Get("/galgame/:gid/collections/mine", a.GalgameCollectionHandler.MyCollectionsForGalgame)
	authed.Put("/galgame/:gid/collections", a.GalgameCollectionHandler.SetMembership)

	// Community-backed comment WRITES (`/comments` plural). Community is now the
	// unconditional comment backend (the legacy /galgame/:gid/comment writes were
	// retired in charter step 06a); an unconfigured client degrades writes to 503.
	// The anonymous read half is mounted before the auth boundary (see optAuth).
	a.GalgameCommunityCommentHandler.RegisterWrites(authed)

	// Community-backed resource comment WRITES (rating / website / toolset /
	// galgame-resource / quiz `/comments` create + region-aware delete).
	// Post-addressed edit / like / flag are NOT re-registered here — those reuse the
	// galgame `/galgame/comments/:postId*` routes above (region-agnostic; charter
	// deliverable C).
	a.ResourceCommentHandler.RegisterWrites(authed)

	// Galgame resource (authenticated, local)
	authed.Post("/galgame/:gid/resource", a.GalgameResourceHandler.CreateResource)
	authed.Put("/galgame/:gid/resource", a.GalgameResourceHandler.UpdateResource)
	authed.Delete("/galgame/:gid/resource", a.GalgameResourceHandler.DeleteResource)
	authed.Put("/galgame/:gid/resource/like", a.GalgameResourceHandler.ToggleLike)
	authed.Put("/galgame/:gid/resource/valid", a.GalgameResourceHandler.MarkValid)
	authed.Put("/galgame/:gid/resource/expired", a.GalgameResourceHandler.MarkExpired)

	// Galgame rating (authenticated, local)
	authed.Post("/galgame-rating", a.GalgameRatingHandler.CreateRating)
	authed.Put("/galgame-rating/:id", a.GalgameRatingHandler.UpdateRating)
	authed.Delete("/galgame-rating/:id", a.GalgameRatingHandler.DeleteRating)
	authed.Put("/galgame-rating/:id/like", a.GalgameRatingHandler.ToggleLike)

	// Galgame quiz (答题): author / answer / rate-quality / delete + a self
	// "answered" history. Publishing is open to anyone in MVP (no review gate).
	authed.Get("/galgame-quiz/mine/answered", a.GalgameQuizHandler.GetMyAnswered)
	authed.Get("/galgame-quiz/mine/favorites", a.GalgameQuizHandler.GetMyFavorites)
	authed.Post("/galgame-quiz", a.GalgameQuizHandler.CreateQuiz)
	authed.Delete("/galgame-quiz/:id", a.GalgameQuizHandler.DeleteQuiz)
	authed.Post("/galgame-quiz/:id/answer", a.GalgameQuizHandler.AnswerQuiz)
	authed.Put("/galgame-quiz/:id/quality", a.GalgameQuizHandler.RateQuizQuality)
	authed.Put("/galgame-quiz/:id/favorite", a.GalgameQuizHandler.ToggleQuizFavorite)
	// Edit (author or moderator): fetch the full quiz, then update it.
	authed.Get("/galgame-quiz/:id/edit", a.GalgameQuizHandler.GetQuizForEdit)
	authed.Put("/galgame-quiz/:id", a.GalgameQuizHandler.UpdateQuiz)

	// Galgame galgame writes (authenticated + token forwarding).
	//
	// Note on write traversal: the kun_session architecture keeps the OAuth
	// access token opaque to the browser (it lives in Redis; the browser only
	// has the session cookie), so every upstream write must traverse kungal
	// for the middleware to attach the session-stored bearer. The editing
	// engine's BFF (edit_handler) is the one write channel left; the old-wire
	// PR writes retired in E3b and the generic write proxy in wave 169.
	// POST /galgame — the wiki's "admin direct publish" bypass — is gone. It
	// existed because publishing and submitting were two different writes; they
	// are now one lifecycle, so a moderator files a submission through the same
	// wizard as everyone else and approves it from the queue. Two writes, both
	// recorded, instead of one that left no trace of who published what.
	// Editing engine (E3a): the schema-driven editor + the kungal review
	// queue over the generic edit face (S2S actor assertion — see
	// galgame/handler/edit_handler.go). The entry gates are exactly that —
	// entries; field-level adjudication rights come from the engine's own
	// policy (admin/ren hold edit.galgame.game.review). The proposal-directed
	// review surfaces are auth-only since E3b: the handler admits moderators
	// AND the game's creator (owner-review — the engine's kungal overlay
	// grants owners the default keys only).
	authed.Get("/galgame/:gid/edit/bootstrap", a.GalgameEditHandler.Bootstrap)
	authed.Post("/galgame/:gid/edit/proposals", a.GalgameEditHandler.Submit)
	authed.Get("/galgame-edit/mine", a.GalgameEditHandler.Mine)
	authed.Post("/galgame-edit/proposals/:id/withdraw", a.GalgameEditHandler.Withdraw)
	// INFRA-PROXY (mirrors infra key `galgame.review`, moderator+): the queue
	// read + proposal view are proxied to the editing engine; RequireModerator
	// is the entry mirror, the engine's projection holds the real policy.
	authed.Get("/galgame-edit/queue", middleware.RequireModerator(), a.GalgameEditHandler.Queue)
	authed.Get("/galgame-edit/proposals/:id", a.GalgameEditHandler.ProposalDetail)
	authed.Post("/galgame-edit/proposals/:id/amend", a.GalgameEditHandler.Amend)
	authed.Post("/galgame-edit/proposals/:id/merge", a.GalgameEditHandler.Merge)
	authed.Post("/galgame-edit/proposals/:id/decline", a.GalgameEditHandler.Decline)
	// Engine-backed revert (E3b — the old wire's owner-or-admin revert moved
	// onto the new chain; the engine gates every restored field).
	authed.Post("/galgame/:gid/edit/revert", a.GalgameEditHandler.Revert)
	// The old-wire editor write proxies (PUT /galgame/:gid, PR
	// submit/merge/decline, revert, links/aliases, contributors) retired in
	// E3b — every kungal edit write flows through the editing engine above.
	//
	// The wiki taxonomy staff lane — the /galgame-taxonomy/{family} picker +
	// read-back pair and the /galgame-tag|official|engine|series write proxies
	// (public create, admin edit/delete) — retired in wave 169: every upstream
	// face behind it 404s since wave-161 P5 and 03 定案 §4 retires the taxonomy
	// console outright rather than re-targeting it. Taxonomy is a curated
	// registry now; the read lanes above (/galgame-tag etc.) stay, served from
	// the catalog. Relation edits on a game go through the editing engine's
	// pickers, which read the same catalog lanes.

	// Toolset (authenticated)
	authed.Post("/toolset", a.ToolsetHandler.Create)
	authed.Put("/toolset/:id", a.ToolsetHandler.Update)
	authed.Delete("/toolset/:id", a.ToolsetHandler.Delete)
	authed.Put("/toolset/:id/practicality", a.ToolsetPracticalityHandler.UpsertPracticality)
	authed.Post("/toolset/:id/resource", a.ToolsetResourceHandler.CreateResource)
	authed.Put("/toolset/:id/resource", a.ToolsetResourceHandler.UpdateResource)
	authed.Delete("/toolset/:id/resource", a.ToolsetResourceHandler.DeleteResource)
	authed.Post("/toolset/:id/upload/init", a.ToolsetUploadHandler.UploadInit)
	authed.Post("/toolset/:id/upload/complete", a.ToolsetUploadHandler.UploadComplete)
	authed.Post("/toolset/:id/upload/resume", a.ToolsetUploadHandler.UploadResume)
	authed.Post("/toolset/:id/upload/abort", a.ToolsetUploadHandler.UploadAbort)

	// ════════════════════════════════════════════
	// ADMIN routes (moderator / admin capability)
	// ════════════════════════════════════════════

	// The admin overview/stats surface is PURE-FORUM: gated on admin.dashboard.
	admin := authed.Group("")
	admin.Get("/admin/overview/all", middleware.RequirePermission(perm.AdminDashboard), a.AdminOverviewHandler.GetOverview)
	admin.Get("/admin/overview/stats", middleware.RequirePermission(perm.AdminDashboard), a.AdminOverviewHandler.GetStats)

	// Content moderation: kungal no longer brokers identity (account
	// ban/delete/register all live in the OAuth admin UI), but it DOES own
	// every piece of content a user publishes here — so an admin can preview
	// and one-shot purge a spam user's entire kungal footprint. PURE-FORUM:
	// gated on user.purge_content.
	admin.Get("/admin/user/:id/content-stats", middleware.RequirePermission(perm.UserPurgeContent), a.AdminPurgeHandler.GetUserContentStats)
	admin.Delete("/admin/user/:id/content", middleware.RequirePermission(perm.UserPurgeContent), a.AdminPurgeHandler.PurgeUserContent)

	// Role→permission + user→permission override editors and the audit log
	// (permission-first authz, Phase 2 role layer + Phase 3 user layer). Gated by
	// RequireAdmin — a deliberate ROLE gate, NOT RequirePermission: overrides must
	// never be able to lock admins out of the very surface that repairs overrides
	// (self-lockout prevention), so this meta-surface sits OUTSIDE the overridable
	// system, like the infra-proxy mirrors.
	rolePermAdmin := authed.Group("", middleware.RequireAdmin())
	rolePermAdmin.Get("/admin/role-permissions", a.AdminRolePermissionHandler.GetMatrix)
	rolePermAdmin.Put("/admin/role-permissions/:role", a.AdminRolePermissionHandler.Replace)
	rolePermAdmin.Get("/admin/user-permissions/:uid", a.AdminUserPermissionHandler.GetView)
	rolePermAdmin.Put("/admin/user-permissions/:uid", a.AdminUserPermissionHandler.Replace)
	rolePermAdmin.Get("/admin/permission-audit", a.AdminPermissionAuditHandler.List)

	// Trust & Safety moderator inbox (proxied to the trust admin API with the
	// moderator's own token; site forced to kungal).
	//
	// INFRA-PROXY (mirrors infra key `trust.queue_access`, moderator+): the
	// trust admin API re-checks; RequireModerator here is the fail-fast mirror,
	// NOT a pkg/perm boundary.
	trustAdmin := authed.Group("", middleware.RequireModerator())
	trustAdmin.Get("/admin/trust/review-items", a.TrustHandler.ListReviewItems)
	trustAdmin.Get("/admin/trust/review-items/:id", a.TrustHandler.GetReviewItem)
	trustAdmin.Post("/admin/trust/review-items/:id/claim", a.TrustHandler.ClaimReviewItem)
	trustAdmin.Post("/admin/trust/review-items/:id/decide", a.TrustHandler.DecideReviewItem)

	// Galgame admin: a MIXED group — the submission-review routes are
	// INFRA-PROXY (galgame re-checks), the resource-publish ban is PURE-FORUM, so
	// each route carries its own gate rather than a shared group middleware.
	galgameAdmin := authed.Group("")
	// INFRA-PROXY (mirrors infra key `catalog.claim.review`, moderator+): the
	// submission review queue and its four verdicts. RequireModerator is the
	// ENTRY gate; the registry re-checks the asserted actor against
	// catalog.claim.review, which is granted moderator and up so the queue keeps
	// exactly the staffing it had on the wiki.
	//
	// The queue reads pending CLAIMS, not a message log: an entry leaves it
	// because its state moved, so the list cannot drift from reality the way a
	// read-marker over a message table can.
	galgameAdmin.Get("/admin/galgame/submissions", middleware.RequireModerator(), a.GalgameClaimReviewHandler.PendingQueue)
	galgameAdmin.Post(
		"/admin/galgame/:gid/review",
		middleware.RequireModerator(),
		a.GalgameClaimReviewHandler.Review,
	)
	// PURE-FORUM: the local resource_publish_banned kill-switch (migration 061)
	// is enforced entirely by the forum — gated on galgame.ban_resource_publish.
	galgameAdmin.Put(
		"/admin/galgame/:gid/resource-publish-ban",
		middleware.RequirePermission(perm.GalgameBanResourcePublish),
		a.GalgameResourceHandler.SetResourcePublishBan,
	)

	// Doc admin — PURE-FORUM, gated per verb (doc.create / doc.edit /
	// doc.delete). The admin article LIST + reorder + pin are edit-surface
	// reads/mutations, so they ride doc.edit.
	docAdmin := authed.Group("")
	docAdmin.Get("/admin/doc/article", middleware.RequirePermission(perm.DocEdit), a.DocArticleHandler.GetAdminArticles)
	docAdmin.Post("/doc/article", middleware.RequirePermission(perm.DocCreate), a.DocArticleHandler.CreateArticle)
	docAdmin.Put("/doc/article", middleware.RequirePermission(perm.DocEdit), a.DocArticleHandler.UpdateArticle)
	docAdmin.Put("/doc/article/reorder", middleware.RequirePermission(perm.DocEdit), a.DocArticleHandler.ReorderArticles)
	docAdmin.Put("/doc/article/pin", middleware.RequirePermission(perm.DocEdit), a.DocArticleHandler.SetArticlePin)
	docAdmin.Delete("/doc/article", middleware.RequirePermission(perm.DocDelete), a.DocArticleHandler.DeleteArticle)
	docAdmin.Post("/doc/category", middleware.RequirePermission(perm.DocCreate), a.DocCategoryHandler.CreateCategory)
	docAdmin.Put("/doc/category", middleware.RequirePermission(perm.DocEdit), a.DocCategoryHandler.UpdateCategory)
	docAdmin.Delete("/doc/category", middleware.RequirePermission(perm.DocDelete), a.DocCategoryHandler.DeleteCategory)
	docAdmin.Post("/doc/tag", middleware.RequirePermission(perm.DocCreate), a.DocTagHandler.CreateTag)
	docAdmin.Put("/doc/tag", middleware.RequirePermission(perm.DocEdit), a.DocTagHandler.UpdateTag)
	docAdmin.Delete("/doc/tag", middleware.RequirePermission(perm.DocDelete), a.DocTagHandler.DeleteTag)

	// Website admin — PURE-FORUM, gated per verb (website.create / website.edit
	// / website.delete). Category edit rides website.edit.
	wsAdmin := authed.Group("")
	wsAdmin.Post("/website", middleware.RequirePermission(perm.WebsiteCreate), a.WebsiteHandler.CreateWebsite)
	wsAdmin.Put("/website/:domain", middleware.RequirePermission(perm.WebsiteEdit), a.WebsiteHandler.UpdateWebsite)
	wsAdmin.Delete("/website/:domain", middleware.RequirePermission(perm.WebsiteDelete), a.WebsiteHandler.DeleteWebsite)
	wsAdmin.Put("/website-category", middleware.RequirePermission(perm.WebsiteEdit), a.WebsiteCategoryHandler.UpdateWebsiteCategory)
	wsAdmin.Post("/website-tag", middleware.RequirePermission(perm.WebsiteCreate), a.WebsiteTagHandler.CreateWebsiteTag)
	wsAdmin.Put("/website-tag", middleware.RequirePermission(perm.WebsiteEdit), a.WebsiteTagHandler.UpdateWebsiteTag)
	wsAdmin.Delete("/website-tag", middleware.RequirePermission(perm.WebsiteDelete), a.WebsiteTagHandler.DeleteWebsiteTag)

	// Update admin — PURE-FORUM, gated per verb (update_log.create / .edit /
	// .delete). History + todo share the update-log vocabulary.
	updateAdmin := authed.Group("")
	updateAdmin.Post("/update/history", middleware.RequirePermission(perm.UpdateLogCreate), a.UpdateHandler.CreateHistory)
	updateAdmin.Put("/update/history", middleware.RequirePermission(perm.UpdateLogEdit), a.UpdateHandler.UpdateHistory)
	updateAdmin.Delete("/update/history", middleware.RequirePermission(perm.UpdateLogDelete), a.UpdateHandler.DeleteHistory)
	updateAdmin.Post("/update/todo", middleware.RequirePermission(perm.UpdateLogCreate), a.UpdateHandler.CreateTodo)
	updateAdmin.Put("/update/todo", middleware.RequirePermission(perm.UpdateLogEdit), a.UpdateHandler.UpdateTodo)
	updateAdmin.Delete("/update/todo", middleware.RequirePermission(perm.UpdateLogDelete), a.UpdateHandler.DeleteTodo)

	// Friend-link admin — PURE-FORUM, gated per verb (friend_link.create /
	// .edit / .delete). Drag-reorder rides friend_link.edit.
	friendAdmin := authed.Group("")
	friendAdmin.Post("/admin/friend-link", middleware.RequirePermission(perm.FriendLinkCreate), a.FriendLinkHandler.Create)
	friendAdmin.Put("/admin/friend-link", middleware.RequirePermission(perm.FriendLinkEdit), a.FriendLinkHandler.Update)
	friendAdmin.Delete("/admin/friend-link", middleware.RequirePermission(perm.FriendLinkDelete), a.FriendLinkHandler.Delete)
	friendAdmin.Put("/admin/friend-link/reorder", middleware.RequirePermission(perm.FriendLinkEdit), a.FriendLinkHandler.Reorder)
}
