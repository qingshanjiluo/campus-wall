package repository

import (
	"encoding/json"
	"errors"

	"kun-galgame-api/internal/user/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// StateRepository owns kungal_user_state — the slim local table that holds
// kungal-specific business fields (moemoepoint / daily counters). Identity
// fields (name / avatar / email / bio / status / role) are owned by OAuth and
// must be fetched via pkg/userclient. user_id here = OAuth user.id.
type StateRepository struct {
	db *gorm.DB
}

func NewStateRepository(db *gorm.DB) *StateRepository {
	return &StateRepository{db: db}
}

func (r *StateRepository) DB() *gorm.DB { return r.db }

// Ensure idempotently creates the row for a freshly-seen user. Called from
// the OAuth callback so newly-onboarded users start with the default
// moemoepoint balance and zeroed daily counters.
func (r *StateRepository) Ensure(userID int) error {
	if userID <= 0 {
		return errors.New("invalid userID")
	}
	row := model.KungalUserState{UserID: userID, Moemoepoint: 7}
	return r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error
}

// FindByID returns the state row or sql.ErrNoRows if missing.
func (r *StateRepository) FindByID(userID int) (*model.KungalUserState, error) {
	var s model.KungalUserState
	err := r.db.First(&s, "user_id = ?", userID).Error
	return &s, err
}

// moemoepoint mutations no longer live here: OAuth is the single source of
// truth and changes flow through internal/moemoepoint.Awarder (which mirrors
// the authoritative balance back into this table's moemoepoint cache column).
// kungal_user_state.moemoepoint is now a READ cache only — gating/ranking read
// it; nothing increments it locally.

// LockForUpdate acquires a SELECT ... FOR UPDATE lock on the state row, used
// by interaction paths that read-then-write moemoepoint inside a tx. Replaces
// the old UserRepository.LockUserForUpdate that locked the obsolete user table.
func (r *StateRepository) LockForUpdate(tx *gorm.DB, userID int) (*model.KungalUserState, error) {
	var s model.KungalUserState
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ?", userID).
		First(&s).Error
	return &s, err
}

// CheckIn atomically marks today's check-in, but ONLY when the user hasn't
// already checked in today (daily_check_in = 0). The conditional WHERE makes it
// race-safe with no external lock — concurrent double-clicks can't both pass.
// Returns whether it was applied (false = already checked in). daily_check_in
// is reset to 0 at calendar midnight by the daily cron, so it's the per-day
// gate. The points reward is granted via OAuth (the Awarder), NOT here — this
// only flips the flag (no local moemoepoint +=).
func (r *StateRepository) CheckIn(userID int) (bool, error) {
	res := r.db.Model(&model.KungalUserState{}).
		Where("user_id = ? AND daily_check_in = 0", userID).
		Update("daily_check_in", 1)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}

// UpdateMutedTypes replaces the user's muted notification-category set. The
// caller is responsible for validating keys against the whitelist. Written as
// raw ?::jsonb (mirroring resource_repo.ReplaceProviderNames) so an empty set
// clears the column reliably — a struct/column Update would skip the zero-value
// slice. The model's serializer:json handles the read side.
func (r *StateRepository) UpdateMutedTypes(userID int, keys []string) error {
	if keys == nil {
		keys = []string{}
	}
	data, err := json.Marshal(keys)
	if err != nil {
		return err
	}
	return r.db.Exec(
		`UPDATE kungal_user_state SET muted_notification_types = ?::jsonb WHERE user_id = ?`,
		string(data), userID,
	).Error
}

// IncrementDailyCounter bumps a single daily_* column by 1; used by image /
// toolset upload paths.
func (r *StateRepository) IncrementDailyCounter(userID int, column string) error {
	return r.db.Model(&model.KungalUserState{}).Where("user_id = ?", userID).
		Update(column, gorm.Expr(column+" + 1")).Error
}

// ResetDailyCounters zeros all per-day fields. Run by the daily cron at
// midnight (cron/cron.go), replacing the old UPDATE "user" SET daily_*
// query that touched the obsolete identity table.
func (r *StateRepository) ResetDailyCounters() (int64, error) {
	res := r.db.Exec(`
		UPDATE kungal_user_state SET
			daily_check_in = 0,
			daily_image_count = 0,
			daily_toolset_upload_count = 0,
			daily_toolset_upload_bytes = 0
		WHERE daily_check_in != 0
		   OR daily_image_count != 0
		   OR daily_toolset_upload_count != 0
		   OR daily_toolset_upload_bytes != 0
	`)
	return res.RowsAffected, res.Error
}
