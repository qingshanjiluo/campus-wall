package enforce

import (
	"context"
	"os"
	"testing"

	"kun-galgame-api/internal/trust/dto"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TestApplyIdempotentAndRoutes exercises the dispatcher against the real dev DB
// (trust_disposition_applied): first apply dispatches + records; a replay of the
// same disposition_id is a no-op. Also checks action → adapter routing.
func TestApplyIdempotentAndRoutes(t *testing.T) {
	_ = godotenv.Load("../../../.env")
	dsn := os.Getenv("KUN_DATABASE_URL")
	if dsn == "" {
		t.Skip("KUN_DATABASE_URL not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("db open: %v", err)
	}

	const dispID int64 = 2_000_000_777
	db.Exec("DELETE FROM trust_disposition_applied WHERE disposition_id = ?", dispID)
	defer db.Exec("DELETE FROM trust_disposition_applied WHERE disposition_id = ?", dispID)

	var hideCalls, removeCalls int
	reg := Registry{
		"forum_reply": {
			Hide:     func(_ context.Context, _ int) error { hideCalls++; return nil },
			Remove:   func(_ context.Context, _ int) error { removeCalls++; return nil },
			AuthorID: func(_ context.Context, _ int) (int, error) { return 0, nil },
		},
	}
	svc := NewService(db, reg, nil)

	cb := dto.TrustCallback{DispositionID: dispID, SubjectKind: "forum_reply", SubjectID: "123", Action: ActionHide}

	if err := svc.Apply(context.Background(), cb); err != nil {
		t.Fatalf("first apply: %v", err)
	}
	if hideCalls != 1 {
		t.Fatalf("expected 1 hide call, got %d", hideCalls)
	}
	var recorded bool
	db.Raw("SELECT EXISTS(SELECT 1 FROM trust_disposition_applied WHERE disposition_id = ?)", dispID).Scan(&recorded)
	if !recorded {
		t.Fatal("disposition not recorded")
	}

	// Replay: same disposition_id → no-op (no second hide call).
	if err := svc.Apply(context.Background(), cb); err != nil {
		t.Fatalf("replay apply: %v", err)
	}
	if hideCalls != 1 {
		t.Fatalf("replay must not re-dispatch; hide calls = %d", hideCalls)
	}

	// An unknown subject_kind (no adapter) is a safe no-op, still recorded.
	const dispID2 int64 = 2_000_000_778
	db.Exec("DELETE FROM trust_disposition_applied WHERE disposition_id = ?", dispID2)
	defer db.Exec("DELETE FROM trust_disposition_applied WHERE disposition_id = ?", dispID2)
	if err := svc.Apply(context.Background(), dto.TrustCallback{DispositionID: dispID2, SubjectKind: "user", SubjectID: "5", Action: ActionRemove}); err != nil {
		t.Fatalf("no-adapter apply: %v", err)
	}
	if removeCalls != 0 {
		t.Fatalf("user has no adapter; remove must not be called, got %d", removeCalls)
	}
}
