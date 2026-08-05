package service

import (
	"context"
	"testing"

	"kun-galgame-api/internal/topic/dto"
	"kun-galgame-api/internal/trust/gate"
	"kun-galgame-api/pkg/trustclient"
)

// scriptedChecker is a gate.Checker fake returning a fixed decision.
type scriptedChecker struct{ decision string }

func (c scriptedChecker) Check(_ context.Context, _ trustclient.CheckRequest) (*trustclient.CheckResult, error) {
	return &trustclient.CheckResult{Decision: c.decision, Matched: []string{"坏词"}}, nil
}

// A denied topic create returns the 422-class blocked error and NOTHING is
// persisted: the check sits strictly before the tx, so with nil repos the deny
// path returns without ever dereferencing the DB (an allow would have paniced
// dialing the repo — proof the write was never reached).
func TestCreateTopicDeniedNothingPersisted(t *testing.T) {
	svc := NewTopicWriteService(
		nil, nil, nil, nil, nil, nil,
		gate.NewCheckService(scriptedChecker{decision: gate.DecisionDeny}),
		gate.NewScanService(nil),
	)
	id, appErr := svc.Create(context.Background(), 7, &dto.CreateTopicRequest{
		Title: "标题", Content: "正文", Category: "others", Sections: []string{"闲聊"},
	})
	if appErr == nil {
		t.Fatal("deny must return an error")
	}
	if appErr.StatusCode != 422 {
		t.Fatalf("deny status = %d, want 422", appErr.StatusCode)
	}
	if id != 0 {
		t.Fatalf("deny must persist nothing, got id %d", id)
	}
}

// A denied topic-comment create (trust wave 2) returns the 422-class blocked
// error and touches no DB: with a nil parent the gate sits strictly before any
// repo call, so nil repos are never dereferenced (an allow would have panicked
// dialing the repo — proof the write was never reached).
func TestCreateCommentDeniedNothingPersisted(t *testing.T) {
	svc := NewCommentService(
		nil, nil, nil, nil, nil,
		gate.NewCheckService(scriptedChecker{decision: gate.DecisionDeny}),
		gate.NewScanService(nil),
	)
	resp, appErr := svc.CreateComment(context.Background(), 7, 1, 1, 2, nil, "违禁内容")
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
