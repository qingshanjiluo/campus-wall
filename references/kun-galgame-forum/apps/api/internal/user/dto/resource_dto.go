package dto

// UserResourceItem is an entry in GET /api/user/:userID/resources.
type UserResourceItem struct {
	ID          int         `json:"id"`
	GalgameID   int         `json:"galgame_id"`
	GalgameName KunLanguage `json:"galgame_name"`
	Type        string      `json:"type"`
	Language    string      `json:"language"`
	Platform    string      `json:"platform"`
	Size        string      `json:"size"`
	Link        []string    `json:"link"`
	Code        string      `json:"code"`
	Password    string      `json:"password"`
	Note        string      `json:"note"`
	Status      int         `json:"status"`
	Created     string      `json:"created"`
}

// UserResourcesResponse is the payload for GET /api/user/:userID/resources.
type UserResourcesResponse struct {
	Resources []UserResourceItem `json:"resources"`
	Total     int64              `json:"total"`
}
