package perm

import "testing"

// allPerms is the full 47-key vocabulary, listed explicitly so this test is a
// second, independent copy of the matrix — if a constant is added/removed
// without updating both this list and the bundles, a count assertion trips.
var allPerms = []Permission{
	TopicEditAny, TopicHide, TopicSetBestAnswer,
	ReplyDeleteAny, ReplyPin,
	CommentTopicDelete,
	CommentGalgameEdit, CommentGalgameDelete,
	CommentRatingEdit, CommentRatingDelete,
	CommentWebsiteEdit, CommentWebsiteDelete,
	CommentToolsetEdit, CommentToolsetDelete,
	CommentResourceEdit, CommentResourceDelete,
	CommentQuizEdit, CommentQuizDelete,
	PollCreateAny, PollEditAny, PollDeleteAny, PollViewRestricted,
	GalgameBanResourcePublish,
	QuizEditAny, QuizDeleteAny,
	ResourceEditAny, ResourceDeleteAny,
	RatingDeleteAny,
	ToolsetEditAny, ToolsetDeleteAny,
	ToolsetResourceEditAny, ToolsetResourceDeleteAny, ToolsetUploadBypass,
	DocCreate, DocEdit, DocDelete,
	WebsiteCreate, WebsiteEdit, WebsiteDelete,
	FriendLinkCreate, FriendLinkEdit, FriendLinkDelete,
	UpdateLogCreate, UpdateLogEdit, UpdateLogDelete,
	AdminDashboard, UserPurgeContent,
}

// adminOnly is the pair of site-administration keys that separate admin/ren
// from moderator.
var adminOnly = map[Permission]bool{
	AdminDashboard:   true,
	UserPurgeContent: true,
}

const (
	totalPerms = 47
	modPerms   = 45 // total minus the two admin-only keys
)

// isAdminOnly reports whether p is one of the two admin-tier permissions.
func isAdminOnly(p Permission) bool { return adminOnly[p] }

// TestVocabularySize pins the vocabulary at exactly 47 distinct keys.
func TestVocabularySize(t *testing.T) {
	if len(allPerms) != totalPerms {
		t.Fatalf("allPerms has %d keys, want %d", len(allPerms), totalPerms)
	}
	seen := make(map[Permission]bool, len(allPerms))
	for _, p := range allPerms {
		if p == "" {
			t.Errorf("empty permission string in allPerms")
		}
		if seen[p] {
			t.Errorf("duplicate permission %q", p)
		}
		seen[p] = true
	}
	if len(adminOnly) != totalPerms-modPerms {
		t.Fatalf("adminOnly has %d keys, want %d", len(adminOnly), totalPerms-modPerms)
	}
}

// bundleSet returns the deduplicated permission set a role's bundle grants,
// and fails if the bundle carried a duplicate (which would make the length
// assertions meaningless).
func bundleSet(t *testing.T, roleName string) map[Permission]bool {
	t.Helper()
	set := make(map[Permission]bool)
	for _, p := range Bundles[roleName] {
		if set[p] {
			t.Errorf("role %q bundle lists %q twice", roleName, p)
		}
		set[p] = true
	}
	return set
}

// TestBundleSizes pins the exact grant counts: moderator 45, admin 47, ren 47.
func TestBundleSizes(t *testing.T) {
	cases := []struct {
		role string
		size int
	}{
		{"moderator", modPerms},
		{"admin", totalPerms},
		{"ren", totalPerms},
	}
	for _, tc := range cases {
		if got := len(bundleSet(t, tc.role)); got != tc.size {
			t.Errorf("role %q grants %d perms, want %d", tc.role, got, tc.size)
		}
	}
}

// TestContainment pins moderator ⊂ admin and admin == ren.
func TestContainment(t *testing.T) {
	mod := bundleSet(t, "moderator")
	admin := bundleSet(t, "admin")
	ren := bundleSet(t, "ren")

	// moderator ⊂ admin: every moderator grant is an admin grant.
	for p := range mod {
		if !admin[p] {
			t.Errorf("moderator holds %q but admin does not (containment broken)", p)
		}
	}
	if len(mod) >= len(admin) {
		t.Errorf("moderator (%d) must be a STRICT subset of admin (%d)", len(mod), len(admin))
	}
	// admin == ren.
	if len(admin) != len(ren) {
		t.Fatalf("admin (%d) and ren (%d) differ in size", len(admin), len(ren))
	}
	for p := range admin {
		if !ren[p] {
			t.Errorf("admin holds %q but ren does not (admin != ren)", p)
		}
	}
}

// wantGranted is the golden membership predicate: given a single role name and
// a permission, does that role grant it?
//
//   - moderator: every non-admin-only key.
//   - admin / ren: every key.
//   - user / creator / any unknown name: nothing.
func wantGranted(roleName string, p Permission) bool {
	switch roleName {
	case "moderator":
		return !isAdminOnly(p)
	case "admin", "ren":
		return true
	default: // user, creator, unknown, ...
		return false
	}
}

// TestMatrix table-drives the entire role×permission matrix through Can: every
// one of the 47 keys against user/creator/moderator/admin/ren/unknown, plus the
// empty (plain logged-in user) role set.
func TestMatrix(t *testing.T) {
	singleRoles := []string{"user", "creator", "moderator", "admin", "ren", "unknown"}
	for _, roleName := range singleRoles {
		for _, p := range allPerms {
			got := Can([]string{roleName}, p)
			want := wantGranted(roleName, p)
			if got != want {
				t.Errorf("Can([%s], %q) = %v, want %v", roleName, p, got, want)
			}
		}
	}

	// The empty role set (a plain `user` never carries a claim) grants nothing.
	for _, p := range allPerms {
		if Can(nil, p) {
			t.Errorf("Can(nil, %q) = true, want false (empty roles grant nothing)", p)
		}
		if Can([]string{}, p) {
			t.Errorf("Can([], %q) = true, want false (empty roles grant nothing)", p)
		}
	}
}

// TestUnknownPermission proves an unknown permission key is never granted, even
// to the highest role.
func TestUnknownPermission(t *testing.T) {
	for _, roleName := range []string{"moderator", "admin", "ren"} {
		if Can([]string{roleName}, Permission("does.not.exist")) {
			t.Errorf("role %q granted an unknown permission", roleName)
		}
	}
}

// TestMultiRoleUnion proves Can is an OR across the caller's role set: a claim
// carrying both a non-granting and a granting role still resolves true.
func TestMultiRoleUnion(t *testing.T) {
	roles := []string{"creator", "admin"}
	if !Can(roles, UserPurgeContent) {
		t.Errorf("Can([creator, admin], user.purge_content) = false, want true")
	}
	if !Can([]string{"user", "moderator"}, TopicHide) {
		t.Errorf("Can([user, moderator], topic.hide) = false, want true")
	}
	// but a purely non-granting union still yields false.
	if Can([]string{"user", "creator"}, TopicHide) {
		t.Errorf("Can([user, creator], topic.hide) = true, want false")
	}
}
