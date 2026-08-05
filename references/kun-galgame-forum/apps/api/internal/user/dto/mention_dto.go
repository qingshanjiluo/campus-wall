package dto

// MentionUser is the slim user shape returned by the @mention autocomplete
// endpoint: just what the editor dropdown needs to render a row and build the
// `[@name](kungal-user:<id>)` token. Intentionally omits email / roles /
// status / created_at — autocomplete only needs identity + avatar.
type MentionUser struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
}
