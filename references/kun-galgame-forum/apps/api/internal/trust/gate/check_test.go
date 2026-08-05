package gate

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"kun-galgame-api/pkg/trustclient"
)

// fakeChecker is a hand fake for the Checker interface: it counts dials and
// returns a scripted result/error, optionally blocking until ctx is cancelled
// (to exercise the 500ms fail-open timeout).
type fakeChecker struct {
	calls  int32
	result *trustclient.CheckResult
	err    error
	block  bool
}

func (f *fakeChecker) Check(ctx context.Context, _ trustclient.CheckRequest) (*trustclient.CheckResult, error) {
	atomic.AddInt32(&f.calls, 1)
	if f.block {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return f.result, f.err
}

// A disabled gate (nil checker) never dials and always allows — proving the
// default-off posture makes zero network calls.
func TestCheckDisabledZeroDial(t *testing.T) {
	s := NewCheckService(nil)
	if s.Enabled() {
		t.Fatal("nil checker must be disabled")
	}
	decision, matched := s.Decision(context.Background(), "任何内容", nil)
	if decision != DecisionAllow || matched != nil {
		t.Fatalf("disabled gate = (%q,%v), want allow/nil", decision, matched)
	}
	// A nil *CheckService is also safe.
	var nilSvc *CheckService
	if d, _ := nilSvc.Decision(context.Background(), "x", nil); d != DecisionAllow {
		t.Fatalf("nil service = %q, want allow", d)
	}
}

func TestCheckDecisionMapping(t *testing.T) {
	cases := []struct {
		wire string
		want string
	}{
		{"deny", DecisionDeny},
		{"hold", DecisionHold},
		{"allow", DecisionAllow},
		{"", DecisionAllow}, // unknown wire value defaults to allow
	}
	for _, tc := range cases {
		fc := &fakeChecker{result: &trustclient.CheckResult{Decision: tc.wire, Matched: []string{"m"}}}
		s := NewCheckService(fc)
		author := int64(42)
		decision, matched := s.Decision(context.Background(), "内容", &author)
		if decision != tc.want {
			t.Fatalf("wire %q → %q, want %q", tc.wire, decision, tc.want)
		}
		if len(matched) != 1 || matched[0] != "m" {
			t.Fatalf("matched not propagated: %v", matched)
		}
		if fc.calls != 1 {
			t.Fatalf("expected exactly one dial, got %d", fc.calls)
		}
	}
}

// Any checker error fails OPEN — the write proceeds (allow), matched dropped.
func TestCheckFailOpenOnError(t *testing.T) {
	s := NewCheckService(&fakeChecker{err: errors.New("trust 503")})
	decision, matched := s.Decision(context.Background(), "内容", nil)
	if decision != DecisionAllow || matched != nil {
		t.Fatalf("error → (%q,%v), want allow/nil", decision, matched)
	}
}

// A checker that blocks past the 500ms bound fails OPEN promptly — the request
// path is never stalled by a wedged trust service.
func TestCheckFailOpenOnTimeout(t *testing.T) {
	s := NewCheckService(&fakeChecker{block: true})
	start := time.Now()
	decision, _ := s.Decision(context.Background(), "内容", nil)
	elapsed := time.Since(start)
	if decision != DecisionAllow {
		t.Fatalf("timeout → %q, want allow", decision)
	}
	if elapsed > time.Second {
		t.Fatalf("Decision stalled %v; the 500ms bound did not fire", elapsed)
	}
}
