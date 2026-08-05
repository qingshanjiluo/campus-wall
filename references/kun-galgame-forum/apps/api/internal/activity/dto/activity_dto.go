package dto

import (
	"time"

	"kun-galgame-api/pkg/imageclient"
)

// ──────────────────────────────────────────
// Requests
// ──────────────────────────────────────────

type ActivityRequest struct {
	// Cursor is the opaque keyset position from the previous page's nextCursor;
	// empty = first page. Replaces the old `page` — offset paging duplicated /
	// skipped rows across pages (see repository.FetchFeed).
	Cursor string `query:"cursor"`
	Limit  int    `query:"limit" validate:"min=1,max=50"`
	Type   string `query:"type" validate:"required"`
	// ShowNoResource mirrors the user's 显示设置 → 显示没有下载资源的 Galgame
	// preference. Default false (omitted) hides resource-less galgames, so their
	// GALGAME_CREATION activity is dropped from the feed too.
	ShowNoResource bool `query:"show_no_resource"`
}

type TimelineRequest struct {
	Cursor         string `query:"cursor"`
	Limit          int    `query:"limit" validate:"min=1,max=50"`
	ShowNoResource bool   `query:"show_no_resource"`
}

// TabRequest drives the home-page feed. Types (comma-separated activity kinds —
// the user's configurable tab) takes precedence when set; otherwise Tab selects
// one of the legacy built-in buckets all/topic/galgame/resource/others. The topic
// "kinds" TOPIC_NORMAL / TOPIC_RESOURCE_HELP are pseudo-types the service maps to
// TOPIC_CREATION + a section filter (see service.resolveKinds).
type TabRequest struct {
	Tab            string `query:"tab" validate:"omitempty,oneof=all topic galgame resource others"`
	Types          string `query:"types"`
	Cursor         string `query:"cursor"`
	Limit          int    `query:"limit" validate:"min=1,max=50"`
	ShowNoResource bool   `query:"show_no_resource"`
	// ForceSfw makes the 全部 tab always SFW regardless of the viewer's NSFW
	// setting: the FE sets it for the "全部" tab so NSFW topics (+ their replies/
	// comments) and NSFW galgame-scoped activity never appear in the main stream.
	ForceSfw bool `query:"force_sfw"`
}

// ──────────────────────────────────────────
// Responses
// ──────────────────────────────────────────

type Actor struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
}

type ActivityItem struct {
	// ID is the source row's id (galgame id for galgame-scoped rows). Kept
	// internal (json:"-") — the service uses (Timestamp, Type, ID) to build the
	// keyset nextCursor; clients consume nextCursor, never this.
	ID        int       `json:"-"`
	UniqueID  string    `json:"unique_id"`
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	Actor     Actor     `json:"actor"`
	Link      string    `json:"link"`
	Content   string    `json:"content"`
	// Data is the per-type rich card payload (Activity Streams "object"),
	// discriminated by Type and populated during enrichment. nil for types that
	// have no rich card yet — the FE renders those with the generic card. Kept
	// `any` so each type carries only its own shape (see TopicActivityData).
	Data any `json:"data,omitempty"`
}

// TopicActivityData is the rich-card payload for TOPIC_CREATION: everything the
// feed's topic card shows beyond the envelope (the title lives in Content).
// Covers are /image/<hash> tokens (card shows the first few); Sections render in
// the stat row; the badge flags feed the shared TopicTagGroup; TopReply is the
// most-liked reply (omitted when none).
type TopicActivityData struct {
	TopicID int `json:"topic_id"`
	// Title is the topic title. For TOPIC_CREATION the title is also in Content
	// (the card uses that); the 推话题 (TOPIC_UPVOTE) card reads it here, since
	// its Content carries the push description instead.
	Title       string   `json:"title,omitempty"`
	AuthorID    int      `json:"author_id,omitempty"`
	Excerpt     string   `json:"excerpt"`
	Sections    []string `json:"sections"`
	CoverImages []string `json:"cover_images"`
	// Per-cover-token image metadata (dims + ThumbHash), keyed by the
	// /image/<hash> token in CoverImages — lets the feed card reserve each
	// cover's aspect ratio (no CLS) and blur it up. Output-only; empty when
	// image_service is unconfigured or its thumbhash backfill hasn't run.
	CoverImageMeta map[string]imageclient.ImageMeta `json:"cover_image_meta,omitempty"`
	View           int                              `json:"view"`
	LikeCount      int                              `json:"like_count"`
	FavoriteCount  int                              `json:"favorite_count"`
	ReplyCount     int                              `json:"reply_count"`
	CommentCount   int                              `json:"comment_count"`
	UpvoteTime     *time.Time                       `json:"upvote_time"`
	// Edited is the topic's last edit time (null = never edited); the card shows an
	// edit icon + relative time after the timestamp.
	Edited        *time.Time `json:"edited"`
	HasBestAnswer bool       `json:"has_best_answer"`
	IsPoll        bool       `json:"is_poll"`
	IsNSFW        bool       `json:"is_nsfw"`
	TopReply      *TopReply  `json:"top_reply,omitempty"`
	// BestAnswer is the accepted best-answer reply (omitted when none). Same reply
	// as TopReply (same ReplyID) → the card shows only the best-answer style.
	BestAnswer *TopReply `json:"best_answer,omitempty"`
	// Upvotes are the topic's 推话题 records — all of them (few per topic).
	Upvotes []TopicUpvote `json:"upvotes,omitempty"`
	// LatestActivity is the topic's newest reply/comment (omitted when none).
	LatestActivity *LatestActivity      `json:"latest_activity,omitempty"`
	Reactions      []TopicReactionCount `json:"reactions"`
}

// TopicReactionCount is one reaction key's total on a topic, for the feed card,
// plus up to a few reactor avatars (Reactors, shared not per-viewer — the card
// shows ≤3 + a "+N"). Per-viewer "mine" is NOT here (the feed is shared-cached);
// it's hydrated client-side via GET /topic/interactions/mine.
type TopicReactionCount struct {
	Reaction string  `json:"reaction"`
	Count    int     `json:"count"`
	Reactors []Actor `json:"reactors,omitempty"`
}

// TopReply is a topic's most-liked reply (a short excerpt + its like count),
// shown on the feed's topic card. Only populated when a reply has >0 likes.
type TopReply struct {
	// ReplyID lets the feed card tell whether this reply is also the best answer.
	ReplyID int `json:"reply_id"`
	// Floor lets the card deep-link to the reply (/topic/:id?reply=<floor>).
	Floor     int    `json:"floor"`
	User      Actor  `json:"user"`
	Content   string `json:"content"`
	LikeCount int    `json:"like_count"`
}

// TopicUpvote is one 推话题 record on the feed's topic card — same shape as the
// topic-detail /upvotes records so the FE reuses the same component.
type TopicUpvote struct {
	ID          int       `json:"id"`
	User        Actor     `json:"user"`
	Description string    `json:"description"`
	Created     time.Time `json:"created"`
}

// LatestActivity is a topic's most-recent reply or comment, shown on the feed card
// below the 推话题 records. Kind is "reply" / "comment"; ReplyID is the reply's id
// (0 for a comment), so the card can merge it when it's the best answer / 高赞回复.
type LatestActivity struct {
	Kind    string `json:"kind"`
	ReplyID int    `json:"reply_id"`
	// Floor (reply) / CommentId (comment) let the card deep-link to the target.
	Floor     int       `json:"floor"`
	CommentId int       `json:"comment_id"`
	User      Actor     `json:"user"`
	Content   string    `json:"content"`
	Created   time.Time `json:"created"`
}

// NoteActivityData — extras for the 其他 cards: UPDATE_LOG_CREATION carries the
// release Version; TODO_CREATION carries the completion Status (a pointer so 0 =
// 待处理 is still sent, not omitted). Each card reads only its own field.
type NoteActivityData struct {
	Version string `json:"version,omitempty"`
	Status  *int   `json:"status,omitempty"`
}

// EntityRefActivityData names the parent entity for the toolset / website
// activity cards whose Content is a comment or resource note: the owning
// toolset's name (TOOLSET_RESOURCE_CREATION, TOOLSET_COMMENT_CREATION) or the
// commented website's name (GALGAME_WEBSITE_COMMENT_CREATION). The creation
// cards (TOOLSET_CREATION / GALGAME_WEBSITE_CREATION) carry the name in Content
// directly and need no payload.
type EntityRefActivityData struct {
	ParentName string `json:"parent_name"`
}

// QuizActivityData is the rich-card payload for GALGAME_QUIZ_CREATION (出题): the
// quiz's category / type / difficulty + answer stats. The 题干 is ActivityItem.Content.
type QuizActivityData struct {
	Category      string `json:"category"`
	Type          string `json:"type"`
	Difficulty    int    `json:"difficulty"`
	AnswerCount   int    `json:"answer_count"`
	CorrectCount  int    `json:"correct_count"`
	FavoriteCount int    `json:"favorite_count"`
	// Description is the 题目描述, truncated to a 200-char markdown preview.
	Description string `json:"description"`
}

// SolutionActivityData is the rich-card payload for MESSAGE_SOLUTION (a best
// answer was accepted): the title of the owning topic, so the card can name it
// and link to it. The accepted reply's preview is ActivityItem.Content.
type SolutionActivityData struct {
	TopicTitle string `json:"topic_title"`
	// Floor of the accepted reply → deep-link to it (/topic/:id?reply=<floor>).
	Floor int `json:"floor"`
}

// ReplyActivityData is the rich-card payload for TOPIC_REPLY_CREATION: the title
// of the topic the reply belongs to (shown at the bottom of the card) and, if the
// reply quoted another reply, that quoted reply. The reply body itself is
// ActivityItem.Content (with @/# tokens already resolved to readable text).
type ReplyActivityData struct {
	TopicTitle string `json:"topic_title"`
	// Floor of this reply → deep-link to it (/topic/:id?reply=<floor>).
	Floor       int          `json:"floor"`
	QuotedReply *QuotedReply `json:"quoted_reply,omitempty"`
}

// QuotedReply is the reply this reply quoted (#floor → its body), shown as a
// nested block in the middle of the reply card. Content has its @/# tokens
// already resolved to readable text.
type QuotedReply struct {
	Floor   int    `json:"floor"`
	Content string `json:"content"`
}

// TopicCommentActivityData is the rich-card payload for TOPIC_COMMENT_CREATION —
// a comment on a reply. Shaped like the reply card: the comment body is in
// ActivityItem.Content; QuotedReply is the reply being commented on (被评论的评论);
// TopicTitle anchors it at the bottom.
type TopicCommentActivityData struct {
	TopicTitle string `json:"topic_title"`
	// CommentId → deep-link to the comment (/topic/:id?comment=<id>).
	CommentId   int          `json:"comment_id"`
	QuotedReply *QuotedReply `json:"quoted_reply,omitempty"`
}

// GalgameActivityData is the rich-card payload for galgame-scoped activity
// (creation / edit / PR / comment / rating / resource): the galgame's name +
// cover + a little metadata, all pulled from the galgame brief already fetched
// during enrichment (no extra query). CoverHash resolves to a CDN URL on the FE
// (imageTokenUrl). Release date is nullable (TBA / unknown).
type GalgameActivityData struct {
	Name        string  `json:"name"`
	CoverHash   string  `json:"cover_hash"`
	Language    string  `json:"language"`
	AgeLimit    string  `json:"age_limit"`
	ReleaseDate *string `json:"release_date"`
	// GalgameID lets the FE link / like / favorite without parsing the link.
	GalgameID int `json:"galgame_id,omitempty"`
	// RevisionID is the galgame revision ROW id for GALGAME_EDIT — the legacy input
	// for the id→number diff resolution. RevisionNumber is the per-galgame
	// revision number the diff endpoint's :rev keys on; the card uses it directly
	// when present (>0) and falls back to RevisionID otherwise.
	RevisionID     int `json:"revision_id,omitempty"`
	RevisionNumber int `json:"revision_number,omitempty"`
	// Developer (制作会社) + Intro are set for the GALGAME_CREATION and GALGAME_EDIT
	// cards (which share the info area), from the galgame detail brief. Developer =
	// officials joined with 、; Intro is the preferred-language introduction,
	// truncated for a 3-line preview.
	Developer string `json:"developer,omitempty"`
	Intro     string `json:"intro,omitempty"`
	// Local rollups, only set for the GALGAME_CREATION rich card (omitempty → the
	// FE defaults each to 0). These are global counts (cache-safe); the viewer's
	// own liked/favorited state is NOT here (would break the shared feed cache).
	ResourceCount int `json:"resource_count,omitempty"`
	LikeCount     int `json:"like_count,omitempty"`
	FavoriteCount int `json:"favorite_count,omitempty"`
	// Rating is only set for GALGAME_RATING_CREATION (the rating card).
	Rating *RatingInfo `json:"rating,omitempty"`
	// ParentComment is the comment being replied to — only for
	// GALGAME_COMMENT_CREATION rows that have a parent (被评论的评论).
	ParentComment *CommentContext `json:"parent_comment,omitempty"`
	// Resource is the published resource's spec — only for
	// GALGAME_RESOURCE_CREATION (download link / codes deliberately omitted).
	Resource *GalgameResourceDetails `json:"resource,omitempty"`
}

// CommentContext is a minimal preview of a parent comment (被评论的评论).
type CommentContext struct {
	Content string `json:"content"`
}

// GalgameResourceDetails is the published resource's spec for its feed card —
// everything EXCEPT the download link / 提取码 / 解压码.
type GalgameResourceDetails struct {
	Type      string `json:"type"`
	Language  string `json:"language"`
	Platform  string `json:"platform"`
	Size      string `json:"size"`
	Note      string `json:"note"`
	LikeCount int    `json:"like_count"`
}

// RatingInfo is the GALGAME_RATING_CREATION rich-card payload — the galgame name
// + cover come from the surrounding GalgameActivityData. ShortSummary is blanked
// when SpoilerLevel != "none" so spoilers never cross the boundary.
type RatingInfo struct {
	RatingID     int    `json:"rating_id"`
	Overall      int    `json:"overall"`
	PlayStatus   string `json:"play_status"`
	Recommend    string `json:"recommend"`
	ShortSummary string `json:"short_summary"`
	SpoilerLevel string `json:"spoiler_level"`
	LikeCount    int    `json:"like_count"`
	AuthorID     int    `json:"author_id"`
}
