package model

import "time"

// KungalUserState holds kungal-specific business fields that have no
// place in the OAuth identity service: virtual currency (moemoepoint),
// daily counters used by site-local rate limits, and a creation
// timestamp tracking when the user first appeared on this site.
//
// UserID is the OAuth user.id (post ID-alignment migration). No FK.
type KungalUserState struct {
	UserID                  int   `gorm:"column:user_id;primaryKey" json:"user_id"`
	Moemoepoint             int   `gorm:"default:7" json:"moemoepoint"`
	DailyCheckIn            int   `gorm:"column:daily_check_in;default:0" json:"-"`
	DailyImageCount         int   `gorm:"column:daily_image_count;default:0" json:"-"`
	DailyToolsetUploadCount int   `gorm:"column:daily_toolset_upload_count;default:0" json:"-"`
	DailyToolsetUploadBytes int64 `gorm:"column:daily_toolset_upload_bytes;default:0" json:"-"`
	// MutedNotificationTypes is the opt-out set of muted notification category
	// keys (see migration 053). serializer:json auto-marshals the []string to
	// the jsonb column, so FindByID hands GetUserStatus a ready-to-filter slice.
	MutedNotificationTypes []string  `gorm:"column:muted_notification_types;serializer:json;type:jsonb;default:'[]'" json:"muted_notification_types"`
	CreatedAt              time.Time `gorm:"column:created" json:"created"`
	UpdatedAt              time.Time `gorm:"column:updated" json:"updated"`
}

func (KungalUserState) TableName() string { return "kungal_user_state" }
