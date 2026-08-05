// Package perm is kungal's permission-first authorization vocabulary: the
// named operation capabilities the forum enforces, plus the role→permission
// bundles that decide who holds them. Every PURE-FORUM enforcement point checks
// a Permission via perm.CanUser (the concrete-caller entry point, uid + roles),
// never a role string — the resolver here is the real boundary for these
// operations. Can is the role-level primitive underneath CanUser that the
// bundles, golden tests, and admin matrix use; enforcement points MUST call
// CanUser so per-user overrides (user_overrides.go) are honored.
//
// # Two-class design
//
// The forum's authority splits into two kinds of operation, and only one of
// them lives in this package:
//
//   - PURE-FORUM permissions (the 47 constants below): the forum's own
//     resolver is the authoritative gate. These are the content-moderation and
//     site-management powers whose truth lives entirely here.
//   - INFRA-PROXY operations (deliberately ABSENT here): operations the forum
//     merely forwards to the infra service with the caller's token, where infra
//     re-checks the real permission. The forum's local gate is only a
//     fail-fast/visibility mirror, so it stays on pkg/role (CanModerate /
//     CanAdminister) with an in-code comment naming the infra key it mirrors.
//     Putting a key here for those would falsely claim the forum owns the
//     decision — the truth is infra's. The nine such operations are
//     galgame.create, galgame.review_submission, galgame.review,
//     galgame.edit_decide, galgame.edit_revert, taxonomy.edit / .delete /
//     .revert, and trust.queue_access.
//
// # Bundles (the golden authorization table)
//
// Roles arrive as an UNORDERED SET of OAuth role-claim strings (docs/oauth/
// 11-roles.md). The management axis is inclusive (moderator ⊂ admin ⊂ ren);
// we express it by composing adminPerms from moderatorPerms, so containment
// holds by construction, not by hand-copied lists. `user` and `creator` hold
// NO forum permissions (creator's direct-publish lives entirely in infra's
// submission face — deliberately no forum key), and any role absent from the
// bundle map resolves to nothing. perm_test.go pins every row of this table.
//
// # Runtime overrides (Phase 2 role layer, Phase 3 user layer)
//
// This table is the compiled BASELINE. On top of it sit two admin-managed,
// Postgres-backed override layers applied without a redeploy: a per-ROLE layer
// (overrides.go — grant/revoke individual keys per role) and a per-USER layer
// (user_overrides.go — grant/revoke keys for one concrete user, so even a
// roleless user can hold a key and a personal revoke beats a role grant). Both
// store only the deltas; with none configured the effective table is
// byte-identical to Bundles, so the golden matrix tests keep passing unchanged.
// CanUser composes both layers; Can sees only the role layer.
package perm

// Permission is one named operation capability, e.g. "topic.edit_any". The
// string value is canonical wire vocabulary — never rename one.
type Permission string

// Permission constants. Naming: <domain>.<verb> or <domain>.<object>.<verb>,
// lower-case with snake_case within a segment.
const (
	// TopicEditAny: edit/rewrite any topic.
	TopicEditAny Permission = "topic.edit_any"
	// TopicHide: hide/unhide a topic.
	TopicHide Permission = "topic.hide"
	// TopicSetBestAnswer: set/unset a topic's best answer.
	TopicSetBestAnswer Permission = "topic.set_best_answer"

	// ReplyDeleteAny: delete any reply.
	ReplyDeleteAny Permission = "reply.delete_any"
	// ReplyPin: pin/unpin a reply.
	ReplyPin Permission = "reply.pin"

	// CommentTopicDelete: delete any topic comment.
	CommentTopicDelete Permission = "comment.topic.delete"
	// CommentGalgameEdit: edit any galgame comment.
	CommentGalgameEdit Permission = "comment.galgame.edit"
	// CommentGalgameDelete: delete any galgame comment.
	CommentGalgameDelete Permission = "comment.galgame.delete"
	// CommentRatingEdit: edit any rating comment.
	CommentRatingEdit Permission = "comment.rating.edit"
	// CommentRatingDelete: delete any rating comment.
	CommentRatingDelete Permission = "comment.rating.delete"
	// CommentWebsiteEdit: edit any website comment.
	CommentWebsiteEdit Permission = "comment.website.edit"
	// CommentWebsiteDelete: delete any website comment.
	CommentWebsiteDelete Permission = "comment.website.delete"
	// CommentToolsetEdit: edit any toolset comment.
	CommentToolsetEdit Permission = "comment.toolset.edit"
	// CommentToolsetDelete: delete any toolset comment.
	CommentToolsetDelete Permission = "comment.toolset.delete"
	// CommentResourceEdit: edit any galgame-resource comment.
	CommentResourceEdit Permission = "comment.resource.edit"
	// CommentResourceDelete: delete any galgame-resource comment.
	CommentResourceDelete Permission = "comment.resource.delete"
	// CommentQuizEdit: edit any galgame-quiz comment.
	CommentQuizEdit Permission = "comment.quiz.edit"
	// CommentQuizDelete: delete any galgame-quiz comment.
	CommentQuizDelete Permission = "comment.quiz.delete"

	// PollCreateAny: create a poll on someone else's topic.
	PollCreateAny Permission = "poll.create_any"
	// PollEditAny: edit a poll on someone else's topic.
	PollEditAny Permission = "poll.edit_any"
	// PollDeleteAny: delete a poll on someone else's topic.
	PollDeleteAny Permission = "poll.delete_any"
	// PollViewRestricted: view results/voter records of restricted or
	// anonymous polls.
	PollViewRestricted Permission = "poll.view_restricted"

	// GalgameBanResourcePublish: ban/unban resource publishing under a galgame
	// (the local resource_publish_banned flag, migration 061).
	GalgameBanResourcePublish Permission = "galgame.ban_resource_publish"

	// QuizEditAny: edit any quiz (includes fetching the answer key).
	QuizEditAny Permission = "quiz.edit_any"
	// QuizDeleteAny: delete any quiz.
	QuizDeleteAny Permission = "quiz.delete_any"

	// ResourceEditAny: edit any galgame resource.
	ResourceEditAny Permission = "resource.edit_any"
	// ResourceDeleteAny: delete any galgame resource.
	ResourceDeleteAny Permission = "resource.delete_any"

	// RatingDeleteAny: delete any rating.
	RatingDeleteAny Permission = "rating.delete_any"

	// ToolsetEditAny: edit any toolset.
	ToolsetEditAny Permission = "toolset.edit_any"
	// ToolsetDeleteAny: delete any toolset.
	ToolsetDeleteAny Permission = "toolset.delete_any"
	// ToolsetResourceEditAny: edit any toolset resource.
	ToolsetResourceEditAny Permission = "toolset.resource.edit_any"
	// ToolsetResourceDeleteAny: delete any toolset resource.
	ToolsetResourceDeleteAny Permission = "toolset.resource.delete_any"
	// ToolsetUploadBypass: upload to any toolset (bypassing owner/quota).
	ToolsetUploadBypass Permission = "toolset.upload_bypass"

	// DocCreate: create doc articles/categories/tags.
	DocCreate Permission = "doc.create"
	// DocEdit: edit doc articles/categories/tags (incl. ordering, pinning).
	DocEdit Permission = "doc.edit"
	// DocDelete: delete doc articles/categories/tags.
	DocDelete Permission = "doc.delete"

	// WebsiteCreate: create websites/tags.
	WebsiteCreate Permission = "website.create"
	// WebsiteEdit: edit websites/categories/tags.
	WebsiteEdit Permission = "website.edit"
	// WebsiteDelete: delete websites/tags.
	WebsiteDelete Permission = "website.delete"

	// FriendLinkCreate: create friend links.
	FriendLinkCreate Permission = "friend_link.create"
	// FriendLinkEdit: edit friend links (incl. ordering).
	FriendLinkEdit Permission = "friend_link.edit"
	// FriendLinkDelete: delete friend links.
	FriendLinkDelete Permission = "friend_link.delete"

	// UpdateLogCreate: create update-log/todo entries.
	UpdateLogCreate Permission = "update_log.create"
	// UpdateLogEdit: edit update-log/todo entries.
	UpdateLogEdit Permission = "update_log.edit"
	// UpdateLogDelete: delete update-log/todo entries.
	UpdateLogDelete Permission = "update_log.delete"

	// AdminDashboard: access the admin overview/statistics surface. (admin)
	AdminDashboard Permission = "admin.dashboard"
	// UserPurgeContent: preview and purge all content of a user. (admin)
	UserPurgeContent Permission = "user.purge_content"
)

// moderatorPerms is everything content-moderation staff (moderator ⊂ admin ⊂
// ren) can do in the forum — the 41 "mod"-threshold permissions. admin/ren add
// only the two site-administration keys on top (see adminPerms).
var moderatorPerms = []Permission{
	TopicEditAny,
	TopicHide,
	TopicSetBestAnswer,
	ReplyDeleteAny,
	ReplyPin,
	CommentTopicDelete,
	CommentGalgameEdit,
	CommentGalgameDelete,
	CommentRatingEdit,
	CommentRatingDelete,
	CommentWebsiteEdit,
	CommentWebsiteDelete,
	CommentToolsetEdit,
	CommentToolsetDelete,
	CommentResourceEdit,
	CommentResourceDelete,
	CommentQuizEdit,
	CommentQuizDelete,
	PollCreateAny,
	PollEditAny,
	PollDeleteAny,
	PollViewRestricted,
	GalgameBanResourcePublish,
	QuizEditAny,
	QuizDeleteAny,
	ResourceEditAny,
	ResourceDeleteAny,
	RatingDeleteAny,
	ToolsetEditAny,
	ToolsetDeleteAny,
	ToolsetResourceEditAny,
	ToolsetResourceDeleteAny,
	ToolsetUploadBypass,
	DocCreate,
	DocEdit,
	DocDelete,
	WebsiteCreate,
	WebsiteEdit,
	WebsiteDelete,
	FriendLinkCreate,
	FriendLinkEdit,
	FriendLinkDelete,
	UpdateLogCreate,
	UpdateLogEdit,
	UpdateLogDelete,
}

// adminPerms adds the two site-administration powers on top of the moderator
// bundle. Composing from moderatorPerms makes the management-axis containment
// (moderator ⊆ admin ⊆ ren) hold by construction.
var adminPerms = append(append([]Permission{}, moderatorPerms...), AdminDashboard, UserPurgeContent)

// Bundles is the forum's role→permission table. Only the three grantable
// management roles appear; `user` and `creator` are intentionally absent (they
// hold no forum permission), and any unknown role resolves to nothing.
var Bundles = map[string][]Permission{
	"moderator": moderatorPerms,
	"admin":     adminPerms,
	"ren":       adminPerms,
}

// Can reports whether ANY of the caller's roles grants permission p. roles is
// the raw OAuth role-claim slice (the same input pkg/role takes). Fail-closed:
// nil/empty roles, a role not in the bundles, and a permission granted by no
// role all yield false.
//
// Can consults the CURRENT effective table (overrides.go): the compiled baseline
// above with the admin-managed runtime override layer applied. With no overrides
// configured the table is byte-identical to Bundles, so behavior matches the
// golden matrix exactly. The table is swapped atomically, so Can is safe to call
// concurrently with a refresh.
func Can(roles []string, p Permission) bool {
	return current.Load().can(roles, p)
}

// resolverT is the tiny flat authorization engine: a role→permission-set map
// answering "does any of these roles grant p?". A local copy of the infra
// authz engine's shape — the forum is a single flat domain, so it lives right
// here in pkg/perm rather than a separate package. Instances are built by
// buildResolver (overrides.go) from the baseline plus any runtime overrides, and
// are immutable once installed (SetOverrides swaps in a fresh one).
type resolverT struct {
	grants map[string]map[Permission]struct{}
}

func (r *resolverT) can(roles []string, p Permission) bool {
	for _, roleName := range roles {
		if set, ok := r.grants[roleName]; ok {
			if _, ok := set[p]; ok {
				return true
			}
		}
	}
	return false
}
