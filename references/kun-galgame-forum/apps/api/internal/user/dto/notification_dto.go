package dto

// NotificationPreferenceResponse carries the caller's opt-out set of muted
// notification-category keys (migration 053). An empty slice = receive
// everything (the default).
type NotificationPreferenceResponse struct {
	MutedTypes []string `json:"muted_types"`
}

// UpdateNotificationPreferenceRequest replaces the caller's muted set wholesale.
// Unknown keys are dropped server-side, so no per-key validation is needed here.
type UpdateNotificationPreferenceRequest struct {
	MutedTypes []string `json:"muted_types"`
}
