package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
)

// ImageTokens is an ordered list of /image/<hash> content tokens persisted as a
// JSON-array string in a SCALAR text column (e.g. topic.cover_images), e.g.
// `["/image/<64hex>","/image/<64hex>"]`. Empty list ⇄ "" on disk.
//
// It is deliberately NOT a Postgres text[]/jsonb: the daily reference-ping cron
// (internal/infrastructure/cron/reference_ping.go) keeps forum-referenced images
// alive by scanning every text/varchar/char column for /image/<hash> tokens, and
// array/jsonb columns are invisible to that schema-derived scan. Keeping the
// tokens in a plain text column means the images they point at stay alive for
// free, with no column-list to maintain. See migration 029.
type ImageTokens []string

// Value marshals to a JSON array string ("" for empty, so the ref-ping
// `LIKE '%/image/%'` pre-filter skips token-less rows).
func (t ImageTokens) Value() (driver.Value, error) {
	if len(t) == 0 {
		return "", nil
	}
	b, err := json.Marshal([]string(t))
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

// Scan reads the JSON-array text back into the slice, tolerating NULL / "" as an
// empty list.
func (t *ImageTokens) Scan(src any) error {
	var s string
	switch v := src.(type) {
	case nil:
		*t = nil
		return nil
	case string:
		s = v
	case []byte:
		s = string(v)
	default:
		return fmt.Errorf("ImageTokens: unsupported Scan type %T", src)
	}
	if strings.TrimSpace(s) == "" {
		*t = nil
		return nil
	}
	var out []string
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return err
	}
	*t = out
	return nil
}
