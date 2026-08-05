package dto

// UserContentStats is the per-type breakdown of a user's kungal content,
// used both to preview a purge (GET) and to report what the LOCAL delete
// removed (DELETE). The four legacy comment tables were dropped in charter step
// 06a (migration 060); comments now live on the community primitive and are
// reported / purged separately via CommunityPosts + the community AuthorPurge.
type UserContentStats struct {
	Topics           int64 `json:"topics"`
	Replies          int64 `json:"replies"`
	TopicComments    int64 `json:"topic_comments"`
	Ratings          int64 `json:"ratings"`
	Resources        int64 `json:"resources"`
	Websites         int64 `json:"websites"`
	Toolsets         int64 `json:"toolsets"`
	ToolsetResources int64 `json:"toolset_resources"`
	ChatMessages     int64 `json:"chat_messages"`
	Messages         int64 `json:"messages"`
	Interactions     int64 `json:"interactions"`
	Total            int64 `json:"total"`
	// CommunityPosts is the user's visible community-primitive posts (all comment
	// areas) — surfaced only on the PREVIEW; the community content is not in the
	// local tables above. Best-effort (0 when the community face is unreachable).
	CommunityPosts int64 `json:"community_posts"`
}

// PurgeResult reports what a purge removed: the local per-type breakdown plus
// the community primitive's compliance-purge counts (posts tombstoned/scrubbed
// and reaction rows deleted this run; both 0 on an idempotent re-run).
type PurgeResult struct {
	UserContentStats
	CommunityPostsPurged      int64 `json:"community_posts_purged"`
	CommunityReactionsDeleted int64 `json:"community_reactions_deleted"`
}
