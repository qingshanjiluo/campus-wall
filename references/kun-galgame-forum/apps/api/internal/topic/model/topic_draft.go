package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// StringSlice is a JSON-array string persisted in a scalar text column ("" ⇄
// empty), used for a draft's tag / section lists. Same storage mechanism as
// ImageTokens, minus the image semantics.
type StringSlice []string

func (s StringSlice) Value() (driver.Value, error) {
	if len(s) == 0 {
		return "", nil
	}
	b, err := json.Marshal([]string(s))
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func (s *StringSlice) Scan(src any) error {
	var str string
	switch v := src.(type) {
	case nil:
		*s = nil
		return nil
	case string:
		str = v
	case []byte:
		str = string(v)
	default:
		return fmt.Errorf("StringSlice: unsupported Scan type %T", src)
	}
	if strings.TrimSpace(str) == "" {
		*s = nil
		return nil
	}
	var out []string
	if err := json.Unmarshal([]byte(str), &out); err != nil {
		return err
	}
	*s = out
	return nil
}

// TopicDraft is a per-user saved draft of an unpublished topic — private to the
// author (scoped by user_id in the service). It holds the same fields as a
// create-topic request so a draft loads straight back into the editor. content
// and cover_images are text columns carrying /image/<hash> tokens, so the daily
// reference-ping cron keeps drafted images alive (see migration 029 + 042).
type TopicDraft struct {
	ID          int         `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID      int         `gorm:"column:user_id;not null" json:"user_id"`
	Title       string      `gorm:"type:varchar(233);not null;default:''" json:"title"`
	Content     string      `gorm:"type:text;not null;default:''" json:"content"`
	Category    string      `gorm:"not null;default:''" json:"category"`
	Sections    StringSlice `gorm:"column:sections;type:text;not null;default:''" json:"sections"`
	CoverImages ImageTokens `gorm:"column:cover_images;type:text;not null;default:''" json:"cover_images"`
	IsNSFW      bool        `gorm:"column:is_nsfw;default:false" json:"is_nsfw"`

	CreatedAt time.Time `gorm:"column:created" json:"created"`
	UpdatedAt time.Time `gorm:"column:updated" json:"updated"`
}

func (TopicDraft) TableName() string { return "topic_draft" }
