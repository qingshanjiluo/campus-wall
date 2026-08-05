package service

import (
	"context"
	"testing"

	"kun-galgame-api/internal/toolset/dto"
	"kun-galgame-api/internal/trust/gate"
	"kun-galgame-api/pkg/trustclient"
)

// scriptedChecker is a gate.Checker fake returning a fixed decision.
type scriptedChecker struct{ decision string }

func (c scriptedChecker) Check(_ context.Context, _ trustclient.CheckRequest) (*trustclient.CheckResult, error) {
	return &trustclient.CheckResult{Decision: c.decision, Matched: []string{"坏词"}}, nil
}

// A denied toolset create (trust wave 2) returns the 422-class blocked error and
// touches no DB: the word-list gate is the very first thing Create does, before
// any repo/tx, so nil repos are never dereferenced (an allow would have panicked
// dialing the repo — proof the write was never reached).
func TestCreateToolsetDeniedNothingPersisted(t *testing.T) {
	svc := NewToolsetService(
		nil, nil, nil, nil, nil, nil, nil,
		gate.NewCheckService(scriptedChecker{decision: gate.DecisionDeny}),
		gate.NewScanService(nil),
	)
	resp, appErr := svc.Create(context.Background(), 7, &dto.CreateToolsetRequest{
		Name: "某工具", Description: "简介", Type: "translator",
		Version: "1.0.0", Aliases: []string{"别名"},
	})
	if appErr == nil {
		t.Fatal("deny must return an error")
	}
	if appErr.StatusCode != 422 {
		t.Fatalf("deny status = %d, want 422", appErr.StatusCode)
	}
	if resp != nil {
		t.Fatalf("deny must persist nothing, got %+v", resp)
	}
}
