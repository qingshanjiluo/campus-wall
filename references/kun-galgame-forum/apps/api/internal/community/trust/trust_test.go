package trust

import (
	"testing"
	"time"

	"kun-galgame-api/pkg/communityclient"
)

// TestBoostForRoles pins the role→boost mapping: staff for any management role
// (moderator/admin/ren), none for a plain user or a creator (kungal declares no
// creator boost — charter ruling 6).
func TestBoostForRoles(t *testing.T) {
	cases := []struct {
		name  string
		roles []string
		want  int32
	}{
		{"moderator", []string{"moderator"}, communityclient.BoostStaff},
		{"admin", []string{"admin"}, communityclient.BoostStaff},
		{"ren", []string{"ren"}, communityclient.BoostStaff},
		{"creator not staff", []string{"creator"}, communityclient.BoostNone},
		{"plain user", nil, communityclient.BoostNone},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := boostForRoles(tc.roles); got != tc.want {
				t.Errorf("boostForRoles(%v) = %d, want %d", tc.roles, got, tc.want)
			}
		})
	}
}

// TestIsVeteranAge pins the ≥30d veteran threshold and the zero-time guard.
func TestIsVeteranAge(t *testing.T) {
	now := time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name    string
		created time.Time
		want    bool
	}{
		{"zero time never veteran", time.Time{}, false},
		{"29 days is not yet", now.Add(-29 * 24 * time.Hour), false},
		{"exactly 30 days qualifies", now.Add(-30 * 24 * time.Hour), true},
		{"a year old qualifies", now.Add(-365 * 24 * time.Hour), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isVeteranAge(tc.created, now); got != tc.want {
				t.Errorf("isVeteranAge(%v) = %v, want %v", tc.created, got, tc.want)
			}
		})
	}
}

// TestInactiveReporterIsNoop proves an unconfigured reporter never acts (Boost
// returns without a client call — no panic on nil deps).
func TestInactiveReporterIsNoop(t *testing.T) {
	// nil client → inactive.
	off := New(nil, nil, nil)
	off.Boost(1, []string{"admin"}) // must not panic / must be a no-op

	// A client built from empty config is unconfigured → still inactive.
	unconfigured := New(communityclient.New(communityclient.Config{}), nil, nil)
	if unconfigured.active() {
		t.Fatal("active() true with an unconfigured client")
	}
}
