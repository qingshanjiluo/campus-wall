package handler

import (
	"context"
	stderrors "errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"kun-galgame-api/internal/constants"
	"kun-galgame-api/internal/galgame/client"
	"kun-galgame-api/internal/galgame/repository"
	msgService "kun-galgame-api/internal/message/service"
	"kun-galgame-api/internal/middleware"
	"kun-galgame-api/internal/moemoepoint"
	"kun-galgame-api/pkg/catalogclient"
	"kun-galgame-api/pkg/errors"
	"kun-galgame-api/pkg/response"
	"kun-galgame-api/pkg/role"
	"kun-galgame-api/pkg/userclient"

	"github.com/gofiber/fiber/v3"
)

// EditHandler is the BFF face onto the infra editing engine (E3a; contract
// /api/v1/catalog/edit/*): the schema-driven galgame editor, the kungal
// review queue (amend / merge / decline — the maintainer-edit crown), the
// revision history + diff reads, and the "my proposals" list. kungal
// authenticates its user locally and ASSERTS the actor over the Basic-authed
// S2S channel (the letmoe E1 posture); the engine's kungal site overlay then
// decides every write's fate — proposals never automerge, they land in the
// review queue.
//
// Actor assertion (E3a ruling 3): roles pass through VERBATIM — the edit
// face resolves them through the galgame family's own perm vocabulary
// (admin/ren hold edit.galgame.game.review), so the BFF holds no policy
// logic beyond the entry gates. Trust tier is the conservative staff
// mapping mirroring the community starter-boost floors (staff → 3,
// everyone else → 0; kungal declares no creator boost).
//
// Degradation: an unconfigured catalog client → 503 on every endpoint and
// the frontend hides the entries. Never a local fallback.
//
// Owner-review (E3b): the review surfaces admit the game's CREATOR alongside
// moderators — the BFF asserts is_entity_owner from the galgame row's uid and
// the engine's kungal overlay grants owners the review rule on the
// default-policy fields only (status/vndb_id stay perm-gated).
//
// Decision side effects (E3b ruling 1 — notifications are the product's job,
// the engine stays notification-free): merge/decline notify the proposer via
// the forum's own message system, merge additionally awards the contribution
// moemoepoint and bumps the local resource_update_time; a submit notifies the
// game owner and mirrors onto the activity timeline — all parity with the old
// PR chain, all best-effort (a failure logs a warning, never rolls back the
// decision).
type EditHandler struct {
	catalog       *catalogclient.Client // S2S actor-assertion edit face — the ONE channel this BFF speaks
	galgameClient *client.GalgameClient // best-effort brief enrichment + owner lookup
	users         *userclient.Client    // best-effort attribution enrichment
	notifier      msgService.Notifier   // best-effort decision notifications
	repo          *repository.GalgameRepository
	// owners answers "who submitted this entry", by gid. It is an interface
	// rather than a direct repo read because it is the ONE fact the edit face
	// needs from the forum's own tables, and naming it separates "the author we
	// render" from "the row we store" — the owner-review gate must agree with
	// the author chip, and a narrow port is what keeps that reviewable.
	owners EntryOwners
}

// EntryOwners resolves an entry's submitter. The registry carries none by
// design — a registry row outlives any account — so this is the forum's own
// frozen snapshot (galgame.creator_user_id, migration 066), the same column the
// author chip renders.
type EntryOwners interface {
	// OwnerOf returns the submitter's uid, or 0 when unknown. Unknown fails the
	// owner check CLOSED; moderators are unaffected.
	OwnerOf(gid int) int
}

// repoOwners adapts the galgame repository to EntryOwners.
type repoOwners struct{ repo *repository.GalgameRepository }

func (r repoOwners) OwnerOf(gid int) int {
	if r.repo == nil || gid <= 0 {
		return 0
	}
	row := r.repo.FindLocal(gid)
	if row.CreatorUserID == nil {
		return 0
	}
	return *row.CreatorUserID
}

func NewEditHandler(
	catalog *catalogclient.Client,
	galgameClient *client.GalgameClient, users *userclient.Client,
	notifier msgService.Notifier, repo *repository.GalgameRepository,
) *EditHandler {
	return &EditHandler{
		catalog: catalog, galgameClient: galgameClient,
		users: users, notifier: notifier, repo: repo, owners: repoOwners{repo: repo},
	}
}

// WithOwners swaps the submitter lookup. Only the assembly point and tests use
// it; production always gets the repository adapter.
func (h *EditHandler) WithOwners(owners EntryOwners) *EditHandler {
	h.owners = owners
	return h
}

// editUser is the attribution shape lists carry (contribution attribution
// is a parity hardline — bare uids would be a regression from the old galgame).
type editUser struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
}

// userMap resolves uids to display users, best-effort: a failed OAuth batch
// read degrades to an empty map and the UI falls back to "用户 #id".
func (h *EditHandler) userMap(ctx context.Context, uids map[int]bool) map[int]editUser {
	out := make(map[int]editUser, len(uids))
	if h.users == nil || len(uids) == 0 {
		return out
	}
	ids := make([]int, 0, len(uids))
	for id := range uids {
		if id > 0 {
			ids = append(ids, id)
		}
	}
	resolved, err := h.users.Users(ctx, ids)
	if err != nil {
		slog.Warn("galgame edit: user enrichment failed", "error", err)
		return out
	}
	for id, u := range resolved {
		out[id] = editUser{ID: u.ID, Name: u.Name, Avatar: u.Avatar}
	}
	return out
}

func collectProposalUIDs(items []catalogclient.EditProposal) map[int]bool {
	uids := make(map[int]bool)
	for i := range items {
		uids[int(items[i].ProposerUID)] = true
		if items[i].DecidedByUID != nil {
			uids[int(*items[i].DecidedByUID)] = true
		}
		for _, a := range items[i].Amendments {
			uids[int(a.AmenderUID)] = true
		}
	}
	return uids
}

// catalogSite is the tenant key kungal files edits under — must equal the
// forum OAuth client's oauth_clients.catalog_site binding.
const catalogSite = "kungal"

// entityTypeGame is the entity type kungal's edits target.
//
// It is the REGISTRY's work now, not the wiki's galgame row — same history,
// same proposals, same revision numbers (the rekey moved them), but a different
// ID SPACE. Every id crossing this boundary therefore has to be translated:
// a kungal URL is a gid, the engine speaks registry work ids, and the two
// overlap, so a missed translation links to a different game instead of
// failing.
const entityTypeGame = catalogclient.EntityTypeWork

// fieldKeyPrefix guards the pass-through patch: only this family's keys may
// ride this BFF (the engine re-validates each key against the registry).
const fieldKeyPrefix = catalogclient.FieldKeyPrefix

var errEditDown = errors.New(errors.CodeBiz, "资料库编辑服务暂不可用", http.StatusServiceUnavailable)

// editActor builds the asserted actor for the current session.
func editActor(c fiber.Ctx) (catalogclient.EditActor, *errors.AppError) {
	user := middleware.GetUser(c)
	if user == nil {
		return catalogclient.EditActor{}, errors.ErrAuthExpired()
	}
	var tier int16
	// Not an authorization gate — a trust-tier projection for the S2S actor
	// assertion (staff → 3). Stays on role.CanModerate (identity/tier, not a
	// pkg/perm forum capability).
	if role.CanModerate(user.Roles) {
		tier = 3 // staff (mirrors the community starter-boost staff floor)
	}
	return catalogclient.EditActor{UserID: int64(user.ID), Roles: user.Roles, TrustTier: tier}, nil
}

// ownerOf answers "who created this entry", by gid.
//
// The registry carries no submitter and never will — a registry row outlives
// any account, so owning it is not a fact about it. The forum keeps its own
// answer instead: `galgame.creator_user_id`, the frozen wiki-era submitter
// snapshot (migration 066) that already backs the author chip on every card.
// Reading the SAME column the product renders is the point: an owner-review
// gate that disagreed with the author shown on screen would be impossible to
// explain.
//
// 0 = unknown, which fails the owner check closed. Moderators are unaffected.
func (h *EditHandler) ownerOf(_ context.Context, gid int64) int {
	if h.owners == nil {
		return 0
	}
	return h.owners.OwnerOf(int(gid))
}

// isGameOwner reports whether uid created the entry behind a REGISTRY work id.
// Fail-closed: an unresolvable work degrades the owner assertion to false.
func (h *EditHandler) isGameOwner(ctx context.Context, workID, uid int64) bool {
	gid := h.gidOf(ctx, workID)
	if gid == 0 {
		return false
	}
	owner := h.ownerOf(ctx, int64(gid))
	return owner > 0 && int64(owner) == uid
}

// workIDOf translates a kungal gid into the registry work id the engine keys
// on. A gid the registry does not know has no editable entity.
func (h *EditHandler) workIDOf(ctx context.Context, gid int64) (int64, *errors.AppError) {
	if h.galgameClient == nil {
		// No bridge, no editable entity. The edit face degrades as a whole (the
		// frontend hides its entries) rather than falling back to a gid, which
		// would address a different work.
		return 0, errEditDown
	}
	ids, appErr := h.galgameClient.CatalogWorkIDs(ctx, []int{int(gid)})
	if appErr != nil {
		return 0, appErr
	}
	workID, ok := ids[int(gid)]
	if !ok {
		return 0, errors.ErrNotFound("条目不存在")
	}
	return workID, nil
}

// editEntry is one entry resolved from a registry work id into everything the
// side-effect lanes need: the kungal id its rows and URLs are keyed by, the
// author the owner-review gate and the "someone proposed an edit" notice are
// addressed to, and a title for the notice body.
//
// It is resolved ONCE per decision rather than looked up per use — the three
// facts come from two different places (the registry for the title, the forum
// for the author) and reading them separately at three call sites is how they
// start disagreeing.
type editEntry struct {
	GID      int
	OwnerUID int
	Name     string
}

// entryOf resolves a registry work id. A zero GID means kungal does not claim
// the work, and every side effect keyed on it is then correctly skipped.
func (h *EditHandler) entryOf(ctx context.Context, workID int64) editEntry {
	gid := h.gidOf(ctx, workID)
	if gid == 0 {
		return editEntry{}
	}
	entry := editEntry{GID: gid, OwnerUID: h.ownerOf(ctx, int64(gid))}
	if h.galgameClient != nil {
		if rows, appErr := h.galgameClient.CatalogRowsByGIDs(ctx, []int{gid}, "names", "all"); appErr == nil {
			if row, ok := rows[gid]; ok {
				brief := client.CatalogItemToBrief(&row)
				entry.Name = client.BriefName(&brief)
			}
		}
	}
	return entry
}

// gidOf is the reverse: one registry work id home to its gid, 0 when kungal
// does not claim it.
func (h *EditHandler) gidOf(ctx context.Context, workID int64) int {
	if h.galgameClient == nil {
		return 0
	}
	gids, appErr := h.galgameClient.GIDsByCatalogIDs(ctx, []int64{workID})
	if appErr != nil {
		slog.Warn("galgame edit: work id → gid failed", "work", workID, "error", appErr)
		return 0
	}
	return gids[workID]
}

// notifyDecision sends the proposer a forum in-site message about a merge /
// decline. gid — NOT the proposal's entity id — because a forum message links
// by kungal id; a registry id in that column points the notice at a different
// entry, and 0 is the honest "no link" the message system already handles.
func (h *EditHandler) notifyDecision(prop *catalogclient.EditProposal, gid int, senderID int64, kind msgService.NotifyKind, content string) {
	if h.notifier == nil {
		return
	}
	if err := h.notifier.Emit(nil, msgService.Spec{
		SenderID: int(senderID), ReceiverID: int(prop.ProposerUID),
		Kind: kind, Content: content, GalgameID: gid,
	}); err != nil {
		slog.Warn("galgame edit: decision notification failed",
			"proposal", prop.ID, "kind", kind, "error", err)
	}
}

// editError maps a catalogclient edit error onto the house envelope. The
// edit face's 4xx replies carry actionable reasons (validation details,
// policy denials, rebase conflicts) — surface them; transport failures and
// the unconfigured client degrade to 503.
func editError(c fiber.Ctx, err error) error {
	var apiErr *catalogclient.EditAPIError
	switch {
	case stderrors.Is(err, catalogclient.ErrNotConfigured):
		return response.Error(c, errEditDown)
	case stderrors.As(err, &apiErr):
		switch apiErr.Status {
		case http.StatusForbidden:
			return response.Error(c, errors.ErrForbidden("你没有权限执行此操作"))
		case http.StatusNotFound:
			return response.Error(c, errors.ErrNotFound("条目或提案不存在"))
		case http.StatusUnprocessableEntity:
			return response.Error(c, errors.ErrValidation(apiErr.Message))
		case http.StatusConflict:
			return response.Error(c, errors.New(errors.CodeBiz,
				"操作冲突（条目已被他人修改或提案已关闭），请刷新后重试", http.StatusConflict))
		}
		slog.Error("galgame edit: upstream error", "status", apiErr.Status, "code", apiErr.Code, "msg", apiErr.Message)
		return response.Error(c, errEditDown)
	default:
		slog.Warn("galgame edit: catalog unreachable", "error", err)
		return response.Error(c, errEditDown)
	}
}

func parseGid(c fiber.Ctx) (int64, *errors.AppError) {
	gid, err := strconv.ParseInt(c.Params("gid"), 10, 64)
	if err != nil || gid <= 0 {
		return 0, errors.ErrBadRequest("无效的 Galgame ID")
	}
	return gid, nil
}

func parseProposalID(c fiber.Ctx) (int64, *errors.AppError) {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.ErrBadRequest("无效的提案 ID")
	}
	return id, nil
}

// queryInt reads a non-negative int query param, returning 0 when absent /
// invalid (the catalog then applies its own default page size).
func queryInt(c fiber.Ctx, key string) int {
	n, err := strconv.Atoi(c.Query(key))
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// ──────────────────────────────────────────
// Editor (game-scoped)
// ──────────────────────────────────────────

// Bootstrap — GET /galgame/:gid/edit/bootstrap (auth). Everything the
// schema-driven editor needs in one read: the entity-aware capability
// projection for THIS user (the UI holds zero policy logic) + the current
// field values keyed by eternal field keys.
func (h *EditHandler) Bootstrap(c fiber.Ctx) error {
	gid, appErr := parseGid(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	actor, appErr := editActor(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	ctx := c.Context()
	workID, appErr := h.workIDOf(ctx, gid)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	// Owner-review (E3b): the entry's creator projects can_review on the
	// default keys — the capability rides the schema projection, the UI
	// holds zero policy logic.
	actor.IsEntityOwner = h.isGameOwner(ctx, workID, actor.UserID)
	values, err := h.catalog.EditSnapshot(ctx, entityTypeGame, workID)
	if err != nil {
		return editError(c, err)
	}
	// The schema projects can_review / would_automerge from the asserted
	// overlay, so it must carry THIS actor's assertion.
	schema, err := h.catalog.GetEditSchema(ctx, catalogSite, entityTypeGame, workID, actor)
	if err != nil {
		return editError(c, err)
	}
	return response.OK(c, fiber.Map{
		"gid":        gid,
		"values":     values,
		"fields":     schema.Fields,
		"can_review": anyReviewable(schema.Fields),
	})
}

func anyReviewable(fields []catalogclient.EditSchemaField) bool {
	for _, f := range fields {
		if f.CanReview {
			return true
		}
	}
	return false
}

// editSubmitRequest carries the dirty-field patch (eternal field keys only —
// the frontend submits exactly what the user changed) and an optional note.
type editSubmitRequest struct {
	Patch map[string]any `json:"patch"`
	Note  string         `json:"note"`
}

// Submit — POST /galgame/:gid/edit/proposals (auth). Files the proposal. On
// kungal a reviewer's own edit direct-merges (automerge=review): admin/ren via
// the review perm, and the game's owner via OwnerReview — result.Merged, the
// change applies immediately. Everyone else's proposal stays open for the queue.
func (h *EditHandler) Submit(c fiber.Ctx) error {
	gid, appErr := parseGid(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	actor, appErr := editActor(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	var req editSubmitRequest
	if err := c.Bind().Body(&req); err != nil {
		return response.Error(c, errors.ErrBadRequest("请求格式错误"))
	}
	if len(req.Patch) == 0 {
		return response.Error(c, errors.ErrValidation("没有需要保存的修改"))
	}
	for key := range req.Patch {
		if !strings.HasPrefix(key, fieldKeyPrefix) {
			return response.Error(c, errors.ErrValidation("提案包含非法字段: "+key))
		}
	}
	if len(req.Note) > 2000 {
		return response.Error(c, errors.ErrValidation("编辑说明过长"))
	}
	// Owner direct-edit (automerge=review): the game's creator reviews the
	// default keys, so — like admin/ren via perm — their own edit applies
	// immediately instead of queuing a proposal against themselves. editActor
	// asserts roles but not ownership; mirror Bootstrap's owner check so the
	// engine sees the same capability. Only for a valid request (avoids the
	// S2S brief lookup on rejected input; the brief is warm from Bootstrap).
	workID, appErr := h.workIDOf(c.Context(), gid)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	actor.IsEntityOwner = h.isGameOwner(c.Context(), workID, actor.UserID)
	result, err := h.catalog.CreateEditProposal(c.Context(), catalogclient.EditCreateRequest{
		EntityType: entityTypeGame, EntityID: workID, Site: catalogSite,
		Patch: req.Patch, Note: req.Note, Actor: actor,
	})
	if err != nil {
		return editError(c, err)
	}
	if !result.Merged {
		h.submitSideEffects(c.Context(), &result.Proposal)
	}
	out := fiber.Map{"merged": result.Merged, "proposal": result.Proposal}
	if result.Revision != nil {
		out["revision"] = result.Revision
	}
	return response.OK(c, out)
}

// submitSideEffects mirrors the old SubmitPR chain onto a new open proposal
// (best-effort): a "requested" notice to the game's owner and a
// GALGAME_PR_CREATION row on the activity timeline. The proposal id continues
// the old galgame PR id space (the E2 transform bumped the sequence past it), so
// wiki_pr_id stays the idempotency key.
func (h *EditHandler) submitSideEffects(ctx context.Context, prop *catalogclient.EditProposal) {
	// EntityID is a REGISTRY id; every row below is keyed by gid, so it comes
	// home first. Writing the work id into galgame_activity.galgame_id would
	// attach the card to whichever entry happens to hold that gid.
	entry := h.entryOf(ctx, prop.EntityID)
	if entry.GID == 0 {
		return
	}
	if h.notifier != nil && entry.OwnerUID > 0 {
		if err := h.notifier.Emit(nil, msgService.Spec{
			SenderID: int(prop.ProposerUID), ReceiverID: entry.OwnerUID,
			Kind: msgService.NotifyRequested, Content: entry.Name,
			GalgameID: entry.GID,
		}); err != nil {
			slog.Warn("galgame edit: requested notification failed", "proposal", prop.ID, "error", err)
		}
	}
	if h.repo != nil {
		if err := h.repo.DB().WithContext(ctx).Exec(`
			INSERT INTO galgame_activity (wiki_pr_id, galgame_id, user_id, type, created)
			VALUES (?, ?, ?, 'GALGAME_PR_CREATION', now())
			ON CONFLICT (wiki_pr_id) DO NOTHING
		`, prop.ID, entry.GID, prop.ProposerUID).Error; err != nil {
			slog.Warn("galgame edit: activity timeline write failed", "proposal", prop.ID, "error", err)
		}
	}
}

// Revisions — GET /galgame/:gid/edit/revisions (optional auth; public like
// the galgame's revision history always was). Includes the E2-migrated history.
// A logged-in reviewer — moderator or the game's creator (E3b) — additionally
// gets can_revert so the history page can offer the revert control.
func (h *EditHandler) Revisions(c fiber.Ctx) error {
	gid, appErr := parseGid(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	workID, appErr := h.workIDOf(c.Context(), gid)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	items, err := h.catalog.ListEditRevisions(c.Context(), entityTypeGame, workID, queryInt(c, "limit"))
	if err != nil {
		return editError(c, err)
	}
	uids := make(map[int]bool)
	for i := range items {
		uids[int(items[i].ActorUID)] = true
		if items[i].AmenderUID != nil {
			uids[int(*items[i].AmenderUID)] = true
		}
	}
	canRevert := false
	if user := middleware.GetUser(c); user != nil {
		// Revert is review-grade: admin ⊂ ren, or the game's owner. Moderators
		// are NOT included — they hold no edit.galgame.game.review, so the
		// engine would reject their revert anyway.
		canRevert = role.CanAdminister(user.Roles) ||
			h.isGameOwner(c.Context(), workID, int64(user.ID))
	}
	return response.OK(c, fiber.Map{
		"gid": gid, "items": items, "users": h.userMap(c.Context(), uids),
		"can_revert": canRevert,
	})
}

// editRevertRequest restores the game to a historical revision.
type editRevertRequest struct {
	ToSeq int    `json:"to_seq"`
	Note  string `json:"note"`
}

// Revert — POST /galgame/:gid/edit/revert (auth; moderator or the game's
// creator — parity with the old wire's owner-or-admin revert). The engine
// enforces the field-level review rule on every restored field, so an owner
// can only revert what they may adjudicate (the kungal default keys).
func (h *EditHandler) Revert(c fiber.Ctx) error {
	gid, appErr := parseGid(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	actor, appErr := editActor(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	var req editRevertRequest
	if err := c.Bind().Body(&req); err != nil {
		return response.Error(c, errors.ErrBadRequest("请求格式错误"))
	}
	if req.ToSeq < 1 {
		return response.Error(c, errors.ErrBadRequest("需要目标版本号"))
	}
	if len(req.Note) > 2000 {
		return response.Error(c, errors.ErrValidation("说明过长"))
	}
	ctx := c.Context()
	workID, appErr := h.workIDOf(ctx, gid)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	actor.IsEntityOwner = h.isGameOwner(ctx, workID, actor.UserID)
	// Review-grade gate: admin ⊂ ren, or the game's owner (mirrors can_revert
	// and the engine's per-field review rule; moderators excluded).
	//
	// INFRA-PROXY mirror: mirrors infra key `edit.galgame.game.review` /
	// `galgame.owner_override` (admin/ren or owner). The engine re-checks every
	// restored field, so this stays a role/owner gate, not a pkg/perm key.
	if !role.CanAdminister(actor.Roles) && !actor.IsEntityOwner {
		return response.Error(c, errors.ErrForbidden("你没有权限执行此操作"))
	}
	result, err := h.catalog.RevertEditEntity(ctx, catalogSite, entityTypeGame, workID, req.ToSeq, req.Note, actor)
	if err != nil {
		return editError(c, err)
	}
	return response.OK(c, result)
}

// Diff — GET /galgame/:gid/edit/diff?from=&to= (public).
func (h *EditHandler) Diff(c fiber.Ctx) error {
	gid, appErr := parseGid(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	from, to := queryInt(c, "from"), queryInt(c, "to")
	if from < 1 || to < 1 {
		return response.Error(c, errors.ErrBadRequest("需要 from/to 版本号"))
	}
	workID, appErr := h.workIDOf(c.Context(), gid)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	diff, err := h.catalog.DiffEditRevisions(c.Context(), entityTypeGame, workID, from, to)
	if err != nil {
		return editError(c, err)
	}
	return response.OK(c, diff)
}

// ──────────────────────────────────────────
// Review queue + my proposals (proposal-scoped)
// ──────────────────────────────────────────

// proposalItem is one enriched list row: the proposal, the kungal id it belongs
// to, and a best-effort brief.
//
// GID is not decoration — it is the ONLY safe way for a client to link to the
// entry. `entity_id` is a registry work id whose space OVERLAPS kungal's, so a
// UI that builds /galgame/{entity_id} lands on a different game and reports no
// error at all. Every list that reaches a template therefore carries the
// translated id, and the templates read that.
type proposalItem struct {
	catalogclient.EditProposal
	GID     int                  `json:"gid"`
	Galgame *client.GalgameBrief `json:"galgame,omitempty"`
}

func (h *EditHandler) enrich(ctx context.Context, items []catalogclient.EditProposal) []proposalItem {
	workIDs := make([]int64, 0, len(items))
	seen := make(map[int64]bool, len(items))
	for i := range items {
		if id := items[i].EntityID; !seen[id] {
			seen[id] = true
			workIDs = append(workIDs, id)
		}
	}
	var gidByWork map[int64]int
	if len(workIDs) > 0 && h.galgameClient != nil {
		var appErr *errors.AppError
		if gidByWork, appErr = h.galgameClient.GIDsByCatalogIDs(ctx, workIDs); appErr != nil {
			slog.Warn("galgame edit: work id → gid enrichment failed", "error", appErr)
		}
	}
	gids := make([]int, 0, len(gidByWork))
	for _, gid := range gidByWork {
		gids = append(gids, gid)
	}
	// The cards show the entry's title. `content_limit=all` because a review
	// queue that cannot see an entry's name cannot review it — the editorial
	// gate is a reader preference, not an authorization.
	var briefs map[int]client.GalgameBrief
	if len(gids) > 0 && h.galgameClient != nil {
		rows, appErr := h.galgameClient.CatalogRowsByGIDs(ctx, gids, "names,covers", "all")
		if appErr != nil {
			slog.Warn("galgame edit: brief enrichment failed", "error", appErr)
		} else {
			briefs = make(map[int]client.GalgameBrief, len(rows))
			for gid := range rows {
				row := rows[gid]
				briefs[gid] = client.CatalogItemToBrief(&row)
			}
		}
	}
	out := make([]proposalItem, 0, len(items))
	for i := range items {
		item := proposalItem{EditProposal: items[i], GID: gidByWork[items[i].EntityID]}
		if b, ok := briefs[item.GID]; ok {
			brief := b
			item.Galgame = &brief
		}
		out = append(out, item)
	}
	return out
}

// Queue — GET /galgame-edit/queue (auth + moderator entry). Open proposals
// on kungal's tenant, newest-first; ?status widens to the decided ones.
// Field-level adjudication rights still come from the engine's projection —
// this gate is the ENTRY, not the policy.
func (h *EditHandler) Queue(c fiber.Ctx) error {
	status := c.Query("status", "open")
	switch status {
	case "open", "merged", "declined", "withdrawn", "":
	default:
		return response.Error(c, errors.ErrBadRequest("未知的提案状态"))
	}
	items, err := h.catalog.ListEditProposals(c.Context(), catalogclient.EditProposalFilter{
		EntityType: entityTypeGame, Site: catalogSite,
		Status: status, Limit: queryInt(c, "limit"),
	})
	if err != nil {
		return editError(c, err)
	}
	return response.OK(c, fiber.Map{
		"items": h.enrich(c.Context(), items),
		"users": h.userMap(c.Context(), collectProposalUIDs(items)),
	})
}

// Mine — GET /galgame-edit/mine (auth). The session user's proposals, all
// states, newest-first; ?gid narrows to one galgame (the editor page's
// "my pending proposal" strip).
func (h *EditHandler) Mine(c fiber.Ctx) error {
	actor, appErr := editActor(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	// ?gid narrows to one entry — a kungal id, so it crosses the bridge before
	// it can name an entity.
	var entityID int64
	if gid := queryInt(c, "gid"); gid > 0 {
		workID, appErr := h.workIDOf(c.Context(), int64(gid))
		if appErr != nil {
			return response.Error(c, appErr)
		}
		entityID = workID
	}
	items, err := h.catalog.ListEditProposals(c.Context(), catalogclient.EditProposalFilter{
		EntityType: entityTypeGame, Site: catalogSite,
		EntityID: entityID, ProposerUID: actor.UserID,
		Status: c.Query("status"), Limit: queryInt(c, "limit"),
	})
	if err != nil {
		return editError(c, err)
	}
	return response.OK(c, fiber.Map{
		"items": h.enrich(c.Context(), items),
		"users": h.userMap(c.Context(), collectProposalUIDs(items)),
	})
}

// proposalForReview loads the proposal and pins it to kungal's own tenant +
// the galgame entity type — this BFF adjudicates nothing else (the S2S site
// binding would reject foreign writes anyway; this keeps reads honest too).
func (h *EditHandler) proposalForReview(ctx context.Context, id int64) (*catalogclient.EditProposal, error) {
	prop, err := h.catalog.GetEditProposal(ctx, id)
	if err != nil {
		return nil, err
	}
	if prop.Site != catalogSite || prop.EntityType != entityTypeGame {
		return nil, &catalogclient.EditAPIError{Status: http.StatusNotFound, Message: "proposal outside the kungal tenant"}
	}
	return prop, nil
}

// stampOwner loads the proposal (pinned to the kungal tenant) and stamps
// is_entity_owner onto the actor — the shared preamble both review gates run
// before applying their own threshold.
func (h *EditHandler) stampOwner(ctx context.Context, id int64, actor *catalogclient.EditActor) (*catalogclient.EditProposal, error) {
	prop, err := h.proposalForReview(ctx, id)
	if err != nil {
		return nil, err
	}
	actor.IsEntityOwner = h.isGameOwner(ctx, prop.EntityID, actor.UserID)
	return prop, nil
}

// reviewEntry authorizes the proposal VIEW surface (ProposalDetail, E3b
// owner-review): the entry admits moderators AND the game's creator; everyone
// else 403s. The owner assertion is stamped onto the actor — the engine's
// kungal overlay holds the field-level policy.
//
// INFRA-PROXY mirror: mirrors infra key `galgame.review` (moderator+, plus the
// owner overlay) — this is a read gate, so the moderator threshold is correct.
// Adjudication is stricter; see decideEntry.
func (h *EditHandler) reviewEntry(ctx context.Context, id int64, actor *catalogclient.EditActor) (*catalogclient.EditProposal, error) {
	prop, err := h.stampOwner(ctx, id, actor)
	if err != nil {
		return nil, err
	}
	if !role.CanModerate(actor.Roles) && !actor.IsEntityOwner {
		return nil, &catalogclient.EditAPIError{Status: http.StatusForbidden, Message: "review entry denied"}
	}
	return prop, nil
}

// decideEntry authorizes the proposal ADJUDICATION surfaces (Amend / Merge /
// Decline): admits admin ⊂ ren AND the game's creator, but NOT a plain
// moderator. This closes a real bug — a bare moderator used to pass this forum
// gate, click merge, then get 403'd by infra, whose edit.galgame.game.review is
// admin/ren-only (the owner overlay grants owners the default-field review).
//
// INFRA-PROXY mirror: mirrors infra key `edit.galgame.game.review` (admin/ren
// or the entity owner).
func (h *EditHandler) decideEntry(ctx context.Context, id int64, actor *catalogclient.EditActor) (*catalogclient.EditProposal, error) {
	prop, err := h.stampOwner(ctx, id, actor)
	if err != nil {
		return nil, err
	}
	if !role.CanAdminister(actor.Roles) && !actor.IsEntityOwner {
		return nil, &catalogclient.EditAPIError{Status: http.StatusForbidden, Message: "decide entry denied"}
	}
	return prop, nil
}

// GameProposals — GET /galgame/:gid/edit/proposals (public — the old wire's
// per-game PR list was a public read). One game's proposals, open by
// default: the owner's per-game review surface and everyone's transparency
// read.
func (h *EditHandler) GameProposals(c fiber.Ctx) error {
	gid, appErr := parseGid(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	status := c.Query("status", "open")
	switch status {
	case "open", "merged", "declined", "withdrawn", "":
	default:
		return response.Error(c, errors.ErrBadRequest("未知的提案状态"))
	}
	workID, appErr := h.workIDOf(c.Context(), gid)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	items, err := h.catalog.ListEditProposals(c.Context(), catalogclient.EditProposalFilter{
		EntityType: entityTypeGame, EntityID: workID, Site: catalogSite,
		Status: status, Limit: queryInt(c, "limit"),
	})
	if err != nil {
		return editError(c, err)
	}
	return response.OK(c, fiber.Map{
		"gid": gid, "items": items,
		"users": h.userMap(c.Context(), collectProposalUIDs(items)),
	})
}

// ProposalDetail — GET /galgame-edit/proposals/:id (auth; moderator or the
// game's creator). The review workbench read: proposal + amendments +
// effective patch, the entity's CURRENT values (per-field old→new compare),
// the reviewer's capability projection, and the galgame brief.
func (h *EditHandler) ProposalDetail(c fiber.Ctx) error {
	id, appErr := parseProposalID(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	actor, appErr := editActor(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	ctx := c.Context()
	prop, err := h.reviewEntry(ctx, id, &actor)
	if err != nil {
		return editError(c, err)
	}
	values, err := h.catalog.EditSnapshot(ctx, entityTypeGame, prop.EntityID)
	if err != nil {
		return editError(c, err)
	}
	schema, err := h.catalog.GetEditSchema(ctx, catalogSite, entityTypeGame, prop.EntityID, actor)
	if err != nil {
		return editError(c, err)
	}
	enriched := h.enrich(ctx, []catalogclient.EditProposal{*prop})
	return response.OK(c, fiber.Map{
		"proposal": enriched[0],
		"values":   values,
		"fields":   schema.Fields,
		"users":    h.userMap(ctx, collectProposalUIDs([]catalogclient.EditProposal{*prop})),
		// can_decide projects the decideEntry predicate for the UI: only
		// admin/ren or the game's owner may amend/merge/decline (a plain
		// moderator can view but not adjudicate). actor.IsEntityOwner was
		// stamped by reviewEntry above.
		"can_decide": role.CanAdminister(actor.Roles) || actor.IsEntityOwner,
	})
}

// editAmendRequest carries the maintainer delta: corrected values (set) and
// rejected fields (unset).
type editAmendRequest struct {
	Set   map[string]any `json:"set"`
	Unset []string       `json:"unset"`
	Note  string         `json:"note"`
}

// Amend — POST /galgame-edit/proposals/:id/amend (auth; moderator or the
// game's creator). The crown mechanism: correct a value / reject a field
// before merging; the merged revision carries proposer + amender double
// attribution.
func (h *EditHandler) Amend(c fiber.Ctx) error {
	id, appErr := parseProposalID(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	actor, appErr := editActor(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	var req editAmendRequest
	if err := c.Bind().Body(&req); err != nil {
		return response.Error(c, errors.ErrBadRequest("请求格式错误"))
	}
	if len(req.Set) == 0 && len(req.Unset) == 0 {
		return response.Error(c, errors.ErrValidation("没有需要修改的内容"))
	}
	for key := range req.Set {
		if !strings.HasPrefix(key, fieldKeyPrefix) {
			return response.Error(c, errors.ErrValidation("提案包含非法字段: "+key))
		}
	}
	if len(req.Note) > 2000 {
		return response.Error(c, errors.ErrValidation("说明过长"))
	}
	ctx := c.Context()
	if _, err := h.decideEntry(ctx, id, &actor); err != nil {
		return editError(c, err)
	}
	amendment, err := h.catalog.AmendEditProposal(ctx, id, req.Set, req.Unset, req.Note, actor)
	if err != nil {
		return editError(c, err)
	}
	return response.OK(c, amendment)
}

type editDecisionRequest struct {
	Note string `json:"note"`
}

// Merge — POST /galgame-edit/proposals/:id/merge (auth; moderator or the
// game's creator).
func (h *EditHandler) Merge(c fiber.Ctx) error {
	id, appErr := parseProposalID(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	actor, appErr := editActor(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	var req editDecisionRequest
	if err := c.Bind().Body(&req); err != nil {
		return response.Error(c, errors.ErrBadRequest("请求格式错误"))
	}
	if len(req.Note) > 2000 {
		return response.Error(c, errors.ErrValidation("说明过长"))
	}
	ctx := c.Context()
	prop, err := h.decideEntry(ctx, id, &actor)
	if err != nil {
		return editError(c, err)
	}
	rev, err := h.catalog.MergeEditProposal(ctx, id, req.Note, actor)
	if err != nil {
		return editError(c, err)
	}
	h.mergeSideEffects(ctx, prop, actor.UserID, rev)
	return response.OK(c, rev)
}

// mergeSideEffects mirrors the old MergePR chain (best-effort, never fails
// the landed merge): the contribution moemoepoint (stable per-proposal
// idempotency key — a merge is exactly-once per proposal; self-merges earn
// nothing, matching the old chain), the local resource_update_time bump so
// the game rises in the update-sorted list, and the "merged" notice to the
// proposer — its content marks a reviewer correction when the merged
// revision carries an amender (E3b ruling 1).
func (h *EditHandler) mergeSideEffects(ctx context.Context, prop *catalogclient.EditProposal, mergerID int64, rev *catalogclient.EditRevision) {
	if prop.ProposerUID != mergerID {
		moemoepoint.Award(int(prop.ProposerUID), constants.RewardPRMerge,
			moemoepoint.ReasonContentApproved, moemoepoint.Ref("galgame_pr", int(prop.EntityID)),
			moemoepoint.Key("galgame_edit_merged", strconv.FormatInt(prop.ID, 10)))
	}
	entry := h.entryOf(ctx, prop.EntityID)
	if h.repo != nil && entry.GID > 0 {
		if err := h.repo.Touch(h.repo.DB().WithContext(ctx), entry.GID); err != nil {
			slog.Warn("galgame edit: resource_update_time bump failed", "gid", entry.GID, "error", err)
		}
	}
	content := entry.Name
	if rev != nil && rev.AmenderUID != nil {
		content = strings.TrimSpace(content + "（审核时有修正）")
	}
	h.notifyDecision(prop, entry.GID, mergerID, msgService.NotifyMerged, content)
}

// Decline — POST /galgame-edit/proposals/:id/decline (auth; moderator or
// the game's creator). The reason is required — a silent decline was the
// old galgame's worst reviewer habit; it travels to the proposer in full on
// the decline notice (E3b ruling 1).
func (h *EditHandler) Decline(c fiber.Ctx) error {
	id, appErr := parseProposalID(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	actor, appErr := editActor(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	var req editDecisionRequest
	if err := c.Bind().Body(&req); err != nil {
		return response.Error(c, errors.ErrBadRequest("请求格式错误"))
	}
	if strings.TrimSpace(req.Note) == "" {
		return response.Error(c, errors.ErrValidation("请填写拒绝理由"))
	}
	if len(req.Note) > 2000 {
		return response.Error(c, errors.ErrValidation("说明过长"))
	}
	ctx := c.Context()
	target, err := h.decideEntry(ctx, id, &actor)
	if err != nil {
		return editError(c, err)
	}
	prop, err := h.catalog.DeclineEditProposal(ctx, id, req.Note, actor)
	if err != nil {
		return editError(c, err)
	}
	entry := h.entryOf(ctx, target.EntityID)
	content := req.Note
	if entry.Name != "" {
		content = entry.Name + "：" + req.Note
	}
	h.notifyDecision(target, entry.GID, actor.UserID, msgService.NotifyDeclined, content)
	return response.OK(c, prop)
}

// Withdraw — POST /galgame-edit/proposals/:id/withdraw (auth). The engine
// enforces proposer-only (ErrNotProposer); no moderator gate here. The tenant
// pre-flight is this BFF's own job on the S2S channel: the asserted-actor face
// will not fence family/tenant for us, so a bare id from another tenant must be
// turned into a 404 here rather than reaching the engine.
func (h *EditHandler) Withdraw(c fiber.Ctx) error {
	id, appErr := parseProposalID(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	actor, appErr := editActor(c)
	if appErr != nil {
		return response.Error(c, appErr)
	}
	ctx := c.Context()
	if _, err := h.proposalForReview(ctx, id); err != nil {
		return editError(c, err)
	}
	prop, err := h.catalog.WithdrawEditProposal(ctx, id, actor)
	if err != nil {
		return editError(c, err)
	}
	return response.OK(c, prop)
}
