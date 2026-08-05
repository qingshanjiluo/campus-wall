package repository

import (
	"fmt"
	"strings"
	"time"

	topicModel "kun-galgame-api/internal/topic/model"

	"gorm.io/gorm"
)

type ActivityRepository struct {
	db *gorm.DB
}

func NewActivityRepository(db *gorm.DB) *ActivityRepository {
	return &ActivityRepository{db: db}
}

// ──────────────────────────────────────────
// Source definitions
// ──────────────────────────────────────────

// knownActivityTypes whitelists the activity types the feed recognises. The
// per-type row projection (content/link/…) lives in the feed_activity triggers
// (migration 034) now, so this set is purely a validator: it filters junk kinds
// out of a custom tab's `types` and bounds the cache-key space.
var knownActivityTypes = map[string]struct{}{
	"TOPIC_CREATION": {}, "TOPIC_REPLY_CREATION": {}, "TOPIC_COMMENT_CREATION": {},
	"TOPIC_UPVOTE": {}, "MESSAGE_UPVOTE": {}, "MESSAGE_SOLUTION": {},
	"GALGAME_CREATION": {}, "GALGAME_COMMENT_CREATION": {}, "GALGAME_RESOURCE_CREATION": {},
	"GALGAME_EDIT": {}, "GALGAME_PR_CREATION": {}, "GALGAME_RATING_CREATION": {},
	"GALGAME_RATING_COMMENT_CREATION": {}, "GALGAME_WEBSITE_CREATION": {},
	"GALGAME_WEBSITE_COMMENT_CREATION": {}, "TOOLSET_CREATION": {},
	"TOOLSET_RESOURCE_CREATION": {}, "TOOLSET_COMMENT_CREATION": {},
	"TODO_CREATION": {}, "UPDATE_LOG_CREATION": {},
	"GALGAME_QUIZ_CREATION": {},
	// The galgame-resource / quiz comment areas (migration 065) project into the
	// feed from the BFF rather than a table trigger — the posts live in the
	// community primitive, so no trigger can fire for them.
	"GALGAME_RESOURCE_COMMENT_CREATION": {}, "GALGAME_QUIZ_COMMENT_CREATION": {},
}

// ──────────────────────────────────────────
// Row projection
// ──────────────────────────────────────────

// ActivityRow is one row of the timeline. Identity (UserName/Avatar) is
// hydrated by the service layer via userclient.
type ActivityRow struct {
	TypeStr   string    `gorm:"column:type_str"`
	ID        int       `gorm:"column:id"`
	Content   string    `gorm:"column:content"`
	Link      string    `gorm:"column:link"`
	Created   time.Time `gorm:"column:created"`
	UserID    int       `gorm:"column:user_id"`
	GalgameID int       `gorm:"column:galgame_id"`
}

// ──────────────────────────────────────────
// Queries
// ──────────────────────────────────────────

// IsKnownType reports whether typeStr is a recognised activity type — used to
// validate a single-type feed request and a custom tab's selected kinds.
func (r *ActivityRepository) IsKnownType(typeStr string) bool {
	_, ok := knownActivityTypes[typeStr]
	return ok
}

// TopicCardData is the per-topic core enrichment for the feed's rich topic card
// (one row from the topic table). Sections/tags/poll/top-reply are separate
// batch loaders below. Excerpt is a server-truncated slice of the body.
type TopicCardData struct {
	ID            int                    `gorm:"column:id"`
	Title         string                 `gorm:"column:title"`
	Excerpt       string                 `gorm:"column:excerpt"`
	CoverImages   topicModel.ImageTokens `gorm:"column:cover_images"`
	View          int                    `gorm:"column:view"`
	LikeCount     int                    `gorm:"column:like_count"`
	FavoriteCount int                    `gorm:"column:favorite_count"`
	ReplyCount    int                    `gorm:"column:reply_count"`
	CommentCount  int                    `gorm:"column:comment_count"`
	IsNSFW        bool                   `gorm:"column:is_nsfw"`
	UpvoteTime    *time.Time             `gorm:"column:upvote_time"`
	BestAnswerID  *int                   `gorm:"column:best_answer_id"`
	AuthorID      int                    `gorm:"column:user_id"`
	Edited        *time.Time             `gorm:"column:edited"`
}

// FetchTopicActivityData batch-loads the topic core row for the given ids (one
// query, no N+1), keyed by id. The body is truncated to a preview in SQL so a
// 100k-char topic never bloats the feed payload. Empty ids → empty map.
func (r *ActivityRepository) FetchTopicActivityData(ids []int) (map[int]TopicCardData, error) {
	out := make(map[int]TopicCardData, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	var rows []TopicCardData
	if err := r.db.Table("topic").
		Select("id, title, user_id, SUBSTRING(content, 1, 300) AS excerpt, cover_images, view, like_count, favorite_count, reply_count, comment_count, is_nsfw, upvote_time, best_answer_id, edited").
		Where("id IN ?", ids).
		Scan(&rows).Error; err != nil {
		return out, err
	}
	for _, row := range rows {
		out[row.ID] = row
	}
	return out, nil
}

// FetchUpvoteTopics maps each topic_upvote row id → its topic id, so the 推话题
// card can pull the pushed topic's data via the topic enrichment. Empty → empty.
func (r *ActivityRepository) FetchUpvoteTopics(upvoteIDs []int) (map[int]int, error) {
	out := map[int]int{}
	if len(upvoteIDs) == 0 {
		return out, nil
	}
	var rows []struct {
		ID      int `gorm:"column:id"`
		TopicID int `gorm:"column:topic_id"`
	}
	if err := r.db.Table("topic_upvote").Select("id, topic_id").
		Where("id IN ?", upvoteIDs).Scan(&rows).Error; err != nil {
		return out, err
	}
	for _, row := range rows {
		out[row.ID] = row.TopicID
	}
	return out, nil
}

// TopicReactionCountRow is one windowed reactor row for the feed cards: the
// (topic, reaction key) with its total count (cnt, repeated on every row) plus
// ONE reactor's user_id — up to feedReactionAvatarCap rows per key.
type TopicReactionCountRow struct {
	TopicID  int    `gorm:"column:topic_id"`
	Reaction string `gorm:"column:reaction"`
	Count    int    `gorm:"column:cnt"`
	UserID   int    `gorm:"column:user_id"`
}

// feedReactionAvatarCap bounds the reactor avatars fetched per (topic, reaction)
// for the feed cards; the rest collapse to a "+N" (mirrors the detail's cap of 3).
const feedReactionAvatarCap = 3

// FetchTopicsReactions batch-loads each topic's reaction keys with their total
// count AND up to feedReactionAvatarCap reactor ids per key (a windowed query),
// so the feed card can render ≤3 avatars + a "+N". Empty ids → empty slice.
func (r *ActivityRepository) FetchTopicsReactions(ids []int) ([]TopicReactionCountRow, error) {
	out := []TopicReactionCountRow{}
	if len(ids) == 0 {
		return out, nil
	}
	err := r.db.Raw(`
		SELECT topic_id, reaction, user_id, cnt FROM (
			SELECT topic_id, reaction, user_id,
				COUNT(*) OVER (PARTITION BY topic_id, reaction) AS cnt,
				ROW_NUMBER() OVER (PARTITION BY topic_id, reaction ORDER BY id) AS rn
			FROM topic_reaction WHERE topic_id IN ?
		) t WHERE rn <= ? ORDER BY topic_id, reaction, rn`, ids, feedReactionAvatarCap).
		Scan(&out).Error
	return out, err
}

// FetchTodoStatuses maps todo id → status (0待处理/1进行中/2已完成/3已废弃) for the
// 其他-tab Note card. Empty ids → empty map.
func (r *ActivityRepository) FetchTodoStatuses(ids []int) (map[int]int, error) {
	out := map[int]int{}
	if len(ids) == 0 {
		return out, nil
	}
	var rows []struct {
		ID     int `gorm:"column:id"`
		Status int `gorm:"column:status"`
	}
	if err := r.db.Table("todo").Select("id, status").
		Where("id IN ?", ids).Scan(&rows).Error; err != nil {
		return out, err
	}
	for _, row := range rows {
		out[row.ID] = row.Status
	}
	return out, nil
}

// FetchUpdateLogVersions maps update_log id → version string for the Note card.
// Empty ids → empty map.
func (r *ActivityRepository) FetchUpdateLogVersions(ids []int) (map[int]string, error) {
	out := map[int]string{}
	if len(ids) == 0 {
		return out, nil
	}
	var rows []struct {
		ID      int    `gorm:"column:id"`
		Version string `gorm:"column:version"`
	}
	if err := r.db.Table("update_log").Select("id, version").
		Where("id IN ?", ids).Scan(&rows).Error; err != nil {
		return out, err
	}
	for _, row := range rows {
		out[row.ID] = row.Version
	}
	return out, nil
}

// TopicCommentContext is the owning topic's title + the reply (被评论的评论) for a
// TOPIC_COMMENT_CREATION card.
type TopicCommentContext struct {
	CommentID    int    `gorm:"column:comment_id"`
	TopicTitle   string `gorm:"column:topic_title"`
	ReplyFloor   int    `gorm:"column:reply_floor"`
	ReplyContent string `gorm:"column:reply_content"`
}

// FetchTopicCommentContext loads, per comment id, the parent reply's floor +
// content and the owning topic's title (one JOIN). Empty ids → empty map.
func (r *ActivityRepository) FetchTopicCommentContext(ids []int) (map[int]TopicCommentContext, error) {
	out := map[int]TopicCommentContext{}
	if len(ids) == 0 {
		return out, nil
	}
	var rows []TopicCommentContext
	if err := r.db.Table("topic_comment c").
		Select(`c.id AS comment_id, tp.title AS topic_title,
			r.floor AS reply_floor, r.content AS reply_content`).
		Joins("JOIN topic_reply r ON r.id = c.topic_reply_id").
		Joins("JOIN topic tp ON tp.id = r.topic_id").
		Where("c.id IN ?", ids).Scan(&rows).Error; err != nil {
		return out, err
	}
	for _, row := range rows {
		out[row.CommentID] = row
	}
	return out, nil
}

// fetchParentNames is the shared id→parent-name lookup for the toolset resource
// card: `childTable c JOIN parentTable p ON p.id = c.<fk>` selecting p.name.
func (r *ActivityRepository) fetchParentNames(childTable, parentTable, fk string, ids []int) (map[int]string, error) {
	out := map[int]string{}
	if len(ids) == 0 {
		return out, nil
	}
	var rows []struct {
		ID   int    `gorm:"column:id"`
		Name string `gorm:"column:name"`
	}
	if err := r.db.Table(childTable+" c").
		Select("c.id, p.name").
		Joins("JOIN "+parentTable+" p ON p.id = c."+fk).
		Where("c.id IN ?", ids).Scan(&rows).Error; err != nil {
		return out, err
	}
	for _, row := range rows {
		out[row.ID] = row.Name
	}
	return out, nil
}

// FetchToolsetResourceParents maps each toolset-resource id → its toolset name.
func (r *ActivityRepository) FetchToolsetResourceParents(ids []int) (map[int]string, error) {
	return r.fetchParentNames("galgame_toolset_resource", "galgame_toolset", "toolset_id", ids)
}

// GalgameResourceRow is one resource's feed-card spec (no download link / codes).
type GalgameResourceRow struct {
	ID        int    `gorm:"column:id"`
	Type      string `gorm:"column:type"`
	Language  string `gorm:"column:language"`
	Platform  string `gorm:"column:platform"`
	Size      string `gorm:"column:size"`
	Note      string `gorm:"column:note"`
	LikeCount int    `gorm:"column:like_count"`
}

// FetchGalgameResourceDetails batch-loads resource specs by id. Empty → empty.
func (r *ActivityRepository) FetchGalgameResourceDetails(ids []int) (map[int]GalgameResourceRow, error) {
	out := map[int]GalgameResourceRow{}
	if len(ids) == 0 {
		return out, nil
	}
	var rows []GalgameResourceRow
	if err := r.db.Table("galgame_resource").
		Select("id, type, language, platform, size, note, like_count").
		Where("id IN ?", ids).Scan(&rows).Error; err != nil {
		return out, err
	}
	for _, row := range rows {
		out[row.ID] = row
	}
	return out, nil
}

// idNameRow is a (topic_id, name) pair for the section/tag batch joins.
type idNameRow struct {
	TopicID int    `gorm:"column:topic_id"`
	Name    string `gorm:"column:name"`
}

func collectIDNames(rows []idNameRow) map[int][]string {
	out := map[int][]string{}
	for _, row := range rows {
		out[row.TopicID] = append(out[row.TopicID], row.Name)
	}
	return out
}

// FetchTopicSections batch-loads section names per topic id (topic_id → names).
func (r *ActivityRepository) FetchTopicSections(ids []int) (map[int][]string, error) {
	if len(ids) == 0 {
		return map[int][]string{}, nil
	}
	var rows []idNameRow
	if err := r.db.Table("topic_section_relation tsr").
		Select("tsr.topic_id, ts.name").
		Joins("JOIN topic_section ts ON ts.id = tsr.topic_section_id").
		Where("tsr.topic_id IN ?", ids).
		Scan(&rows).Error; err != nil {
		return map[int][]string{}, err
	}
	return collectIDNames(rows), nil
}

// FetchTopicPolls returns the set of given topic ids that have a poll attached.
func (r *ActivityRepository) FetchTopicPolls(ids []int) (map[int]bool, error) {
	out := map[int]bool{}
	if len(ids) == 0 {
		return out, nil
	}
	var rows []struct {
		TopicID int `gorm:"column:topic_id"`
	}
	if err := r.db.Table("topic_poll").
		Select("DISTINCT topic_id").
		Where("topic_id IN ?", ids).
		Scan(&rows).Error; err != nil {
		return out, err
	}
	for _, row := range rows {
		out[row.TopicID] = true
	}
	return out, nil
}

// FetchReplyTopicTitles batch-loads the parent topic title for each reply id
// (reply_id → topic.title) — the feed's reply card shows it at the bottom.
func (r *ActivityRepository) FetchReplyTopicTitles(replyIDs []int) (map[int]string, error) {
	out := map[int]string{}
	if len(replyIDs) == 0 {
		return out, nil
	}
	var rows []struct {
		ID    int    `gorm:"column:id"`
		Title string `gorm:"column:title"`
	}
	if err := r.db.Raw(`
		SELECT r.id, t.title
		FROM topic_reply r JOIN topic t ON t.id = r.topic_id
		WHERE r.id IN ?`, replyIDs).Scan(&rows).Error; err != nil {
		return out, err
	}
	for _, row := range rows {
		out[row.ID] = row.Title
	}
	return out, nil
}

// FetchReplyFloors maps reply ids → their floor, so a reply feed card can
// deep-link to /topic/:id?reply=<floor>.
func (r *ActivityRepository) FetchReplyFloors(replyIDs []int) (map[int]int, error) {
	out := map[int]int{}
	if len(replyIDs) == 0 {
		return out, nil
	}
	var rows []struct {
		ID    int `gorm:"column:id"`
		Floor int `gorm:"column:floor"`
	}
	if err := r.db.Raw(`SELECT id, floor FROM topic_reply WHERE id IN ?`, replyIDs).
		Scan(&rows).Error; err != nil {
		return out, err
	}
	for _, row := range rows {
		out[row.ID] = row.Floor
	}
	return out, nil
}

// FetchTopicTitles maps topic ids → their titles, for the best-answer
// (MESSAGE_SOLUTION) feed card, which names the topic it links to.
func (r *ActivityRepository) FetchTopicTitles(topicIDs []int) (map[int]string, error) {
	out := map[int]string{}
	if len(topicIDs) == 0 {
		return out, nil
	}
	var rows []struct {
		ID    int    `gorm:"column:id"`
		Title string `gorm:"column:title"`
	}
	if err := r.db.Raw(`SELECT id, title FROM topic WHERE id IN ?`, topicIDs).
		Scan(&rows).Error; err != nil {
		return out, err
	}
	for _, row := range rows {
		out[row.ID] = row.Title
	}
	return out, nil
}

// ReplyContent is a quoted reply's floor + raw body (tokens unresolved), for the
// feed's reply card.
type ReplyContent struct {
	Floor   int
	Content string
}

// FetchReplyContents batch-loads floor + content for reply ids — the quoted
// replies referenced by feed reply cards (#floor tokens).
func (r *ActivityRepository) FetchReplyContents(replyIDs []int) (map[int]ReplyContent, error) {
	out := map[int]ReplyContent{}
	if len(replyIDs) == 0 {
		return out, nil
	}
	var rows []struct {
		ID      int    `gorm:"column:id"`
		Floor   int    `gorm:"column:floor"`
		Content string `gorm:"column:content"`
	}
	if err := r.db.Raw(`
		SELECT id, floor, content
		FROM topic_reply
		WHERE id IN ?`, replyIDs).Scan(&rows).Error; err != nil {
		return out, err
	}
	for _, row := range rows {
		out[row.ID] = ReplyContent{Floor: row.Floor, Content: row.Content}
	}
	return out, nil
}

// GalgameCounts holds a galgame's local rollups for the new-galgame feed card,
// plus the frozen creator that card's ACTOR is resolved from.
type GalgameCounts struct {
	ResourceCount int
	LikeCount     int
	FavoriteCount int
	// CreatorUserID is the frozen wiki-era submitter (migration 066). The
	// catalog face carries no submitter, so this is the only thing that can
	// tell a GALGAME_CREATION feed row who created the galgame. NULL =
	// unknown, which leaves the row actor-less (rendered as a system event).
	CreatorUserID *int
}

// FetchGalgameCounts batch-loads resource/like/favorite counts + the frozen
// creator for galgame ids from the local galgame table (global — cache-safe).
func (r *ActivityRepository) FetchGalgameCounts(galgameIDs []int) (map[int]GalgameCounts, error) {
	out := map[int]GalgameCounts{}
	if len(galgameIDs) == 0 {
		return out, nil
	}
	var rows []struct {
		ID            int  `gorm:"column:id"`
		ResourceCount int  `gorm:"column:resource_count"`
		LikeCount     int  `gorm:"column:like_count"`
		FavoriteCount int  `gorm:"column:favorite_count"`
		CreatorUserID *int `gorm:"column:creator_user_id"`
	}
	if err := r.db.Raw(`
		SELECT id, resource_count, like_count, favorite_count, creator_user_id
		FROM galgame
		WHERE id IN ?`, galgameIDs).Scan(&rows).Error; err != nil {
		return out, err
	}
	for _, row := range rows {
		out[row.ID] = GalgameCounts{
			ResourceCount: row.ResourceCount,
			LikeCount:     row.LikeCount,
			FavoriteCount: row.FavoriteCount,
			CreatorUserID: row.CreatorUserID,
		}
	}
	return out, nil
}

// EditRevision pairs a GALGAME_EDIT activity's galgame revision row id (global —
// the input for the id→number fallback) with the per-galgame revision NUMBER
// (what the diff endpoint's :rev expects). RevisionNumber is 0 for rows synced
// before the galgame feed started carrying `revision`.
type EditRevision struct {
	RevisionID     int
	RevisionNumber int
}

// FetchEditRevisions maps galgame_activity ids → their galgame revision ref, for the
// feed's edit card to lazily load the diff (directly via the number, or via the
// id→number resolution fallback for legacy rows where the number is unknown).
func (r *ActivityRepository) FetchEditRevisions(activityIDs []int) (map[int]EditRevision, error) {
	out := map[int]EditRevision{}
	if len(activityIDs) == 0 {
		return out, nil
	}
	var rows []struct {
		ID         int  `gorm:"column:id"`
		RevisionID int  `gorm:"column:wiki_revision_id"`
		RevisionNo *int `gorm:"column:wiki_revision_number"`
	}
	// COALESCE because wiki_revision_id is NULL on every engine-fed row (wave
	// 156 N3): those rows are identified by edit_revision_id and always carry a
	// revision NUMBER, so they never need the id→number fallback that this
	// column feeds. Scanning a NULL into the non-pointer int would error out and
	// take the whole timeline page with it.
	if err := r.db.Raw(`
		SELECT id, COALESCE(wiki_revision_id, 0) AS wiki_revision_id, wiki_revision_number
		FROM galgame_activity
		WHERE id IN ?`, activityIDs).Scan(&rows).Error; err != nil {
		return out, err
	}
	for _, row := range rows {
		num := 0
		if row.RevisionNo != nil {
			num = *row.RevisionNo
		}
		out[row.ID] = EditRevision{RevisionID: row.RevisionID, RevisionNumber: num}
	}
	return out, nil
}

// RatingActivity is a galgame rating's feed-card fields. ShortSummary is blanked
// when there's a spoiler so it never leaves the boundary.
type RatingActivity struct {
	Overall      int
	PlayStatus   string
	Recommend    string
	ShortSummary string
	SpoilerLevel string
	LikeCount    int
	AuthorID     int
}

// FetchRatingActivityData batch-loads rating fields by galgame_rating id for the
// feed's rating card. Summaries of spoiler-flagged ratings are dropped.
func (r *ActivityRepository) FetchRatingActivityData(ratingIDs []int) (map[int]RatingActivity, error) {
	out := map[int]RatingActivity{}
	if len(ratingIDs) == 0 {
		return out, nil
	}
	var rows []struct {
		ID           int    `gorm:"column:id"`
		Overall      int    `gorm:"column:overall"`
		PlayStatus   string `gorm:"column:play_status"`
		Recommend    string `gorm:"column:recommend"`
		ShortSummary string `gorm:"column:short_summary"`
		SpoilerLevel string `gorm:"column:spoiler_level"`
		LikeCount    int    `gorm:"column:like_count"`
		UserID       int    `gorm:"column:user_id"`
	}
	if err := r.db.Raw(`
		SELECT id, overall, play_status, recommend, short_summary, spoiler_level, like_count, user_id
		FROM galgame_rating
		WHERE id IN ?`, ratingIDs).Scan(&rows).Error; err != nil {
		return out, err
	}
	for _, row := range rows {
		summary := row.ShortSummary
		if row.SpoilerLevel != "none" {
			summary = "" // never leak a spoiler-flagged summary into the feed
		}
		out[row.ID] = RatingActivity{
			Overall:      row.Overall,
			PlayStatus:   row.PlayStatus,
			Recommend:    row.Recommend,
			ShortSummary: summary,
			SpoilerLevel: row.SpoilerLevel,
			LikeCount:    row.LikeCount,
			AuthorID:     row.UserID,
		}
	}
	return out, nil
}

// QuizActivity is the 出题 card's per-quiz meta (category / type / difficulty +
// answer stats) for the feed.
type QuizActivity struct {
	Category      string
	Type          string
	Difficulty    int
	AnswerCount   int
	CorrectCount  int
	FavoriteCount int
	Description   string
}

// FetchQuizActivityData batch-loads quiz meta by galgame_quiz id for the feed's
// 出题 card. Empty ids → empty map.
func (r *ActivityRepository) FetchQuizActivityData(quizIDs []int) (map[int]QuizActivity, error) {
	out := map[int]QuizActivity{}
	if len(quizIDs) == 0 {
		return out, nil
	}
	var rows []struct {
		ID            int    `gorm:"column:id"`
		Category      string `gorm:"column:category"`
		Type          string `gorm:"column:type"`
		Difficulty    int    `gorm:"column:difficulty"`
		AnswerCount   int    `gorm:"column:answer_count"`
		CorrectCount  int    `gorm:"column:correct_count"`
		FavoriteCount int    `gorm:"column:favorite_count"`
		Description   string `gorm:"column:description"`
	}
	// description is the markdown 题目描述, truncated to a 200-char preview (+ … when
	// clipped) for the feed card; the FE strips the markdown to plain text.
	if err := r.db.Raw(`
		SELECT id, category, type, difficulty, answer_count, correct_count, favorite_count,
			CASE WHEN CHAR_LENGTH(description) > 200
				THEN LEFT(description, 200) || '…'
				ELSE description END AS description
		FROM galgame_quiz
		WHERE id IN ?`, quizIDs).Scan(&rows).Error; err != nil {
		return out, err
	}
	for _, row := range rows {
		out[row.ID] = QuizActivity{
			Category:      row.Category,
			Type:          row.Type,
			Difficulty:    row.Difficulty,
			AnswerCount:   row.AnswerCount,
			CorrectCount:  row.CorrectCount,
			FavoriteCount: row.FavoriteCount,
			Description:   row.Description,
		}
	}
	return out, nil
}

// TopReplyRow is a topic's most-liked reply (excerpt + like count). ID is the
// reply id, so the card can tell whether it's also the best answer.
type TopReplyRow struct {
	TopicID   int    `gorm:"column:topic_id"`
	ID        int    `gorm:"column:id"`
	Floor     int    `gorm:"column:floor"`
	Content   string `gorm:"column:content"`
	LikeCount int    `gorm:"column:like_count"`
	UserID    int    `gorm:"column:user_id"`
}

// FetchTopicTopReply batch-loads each topic's MOST-LIKED reply via DISTINCT ON
// (one row per topic, highest like_count, id as the tiebreaker), restricted to
// replies that actually have likes (>0) so the card only surfaces a notable one.
func (r *ActivityRepository) FetchTopicTopReply(ids []int) (map[int]TopReplyRow, error) {
	out := map[int]TopReplyRow{}
	if len(ids) == 0 {
		return out, nil
	}
	var rows []TopReplyRow
	if err := r.db.Raw(`
		SELECT DISTINCT ON (topic_id) topic_id, id, floor,
			SUBSTRING(content, 1, 200) AS content, like_count, user_id
		FROM topic_reply
		WHERE topic_id IN ? AND like_count > 0
		ORDER BY topic_id, like_count DESC, id DESC`, ids).
		Scan(&rows).Error; err != nil {
		return out, err
	}
	for _, row := range rows {
		out[row.TopicID] = row
	}
	return out, nil
}

// BestAnswerRow is a topic's accepted best-answer reply (excerpt + like count).
// ReplyID is the reply's id, so the card can dedup it against the top reply.
type BestAnswerRow struct {
	TopicID   int    `gorm:"column:topic_id"`
	ReplyID   int    `gorm:"column:reply_id"`
	Floor     int    `gorm:"column:floor"`
	Content   string `gorm:"column:content"`
	LikeCount int    `gorm:"column:like_count"`
	UserID    int    `gorm:"column:user_id"`
}

// FetchTopicBestAnswers batch-loads the accepted best-answer reply (topic
// .best_answer_id → topic_reply) for the topics that have one, keyed by topic id.
func (r *ActivityRepository) FetchTopicBestAnswers(ids []int) (map[int]BestAnswerRow, error) {
	out := map[int]BestAnswerRow{}
	if len(ids) == 0 {
		return out, nil
	}
	var rows []BestAnswerRow
	if err := r.db.Raw(`
		SELECT t.id AS topic_id, r.id AS reply_id, r.floor,
			SUBSTRING(r.content, 1, 200) AS content, r.like_count, r.user_id
		FROM topic t
		JOIN topic_reply r ON r.id = t.best_answer_id
		WHERE t.id IN ? AND t.best_answer_id IS NOT NULL`, ids).
		Scan(&rows).Error; err != nil {
		return out, err
	}
	for _, row := range rows {
		out[row.TopicID] = row
	}
	return out, nil
}

// TopicUpvoteRow is one 推话题 record for the feed card (matches the topic-detail
// /upvotes shape: pusher, one-liner, time). ID is the upvote row id.
type TopicUpvoteRow struct {
	TopicID     int       `gorm:"column:topic_id"`
	ID          int       `gorm:"column:id"`
	UserID      int       `gorm:"column:user_id"`
	Description string    `gorm:"column:description"`
	Created     time.Time `gorm:"column:created"`
}

// FetchTopicUpvotesBatch batch-loads ALL push records for the given topics,
// newest first per topic. Pushes are few per topic (each costs moemoepoint), so
// there's no per-topic cap — the card shows every one. Empty ids → empty map.
func (r *ActivityRepository) FetchTopicUpvotesBatch(ids []int) (map[int][]TopicUpvoteRow, error) {
	out := map[int][]TopicUpvoteRow{}
	if len(ids) == 0 {
		return out, nil
	}
	var rows []TopicUpvoteRow
	if err := r.db.Table("topic_upvote").
		Select("topic_id, id, user_id, description, created").
		Where("topic_id IN ?", ids).
		Order("topic_id, created DESC, id DESC").
		Scan(&rows).Error; err != nil {
		return out, err
	}
	for _, row := range rows {
		out[row.TopicID] = append(out[row.TopicID], row)
	}
	return out, nil
}

// Cursor is the keyset position for the activity feed: the (created, type_str,
// id) of the last row already returned. The feed is a UNION across many source
// tables, so `id` is unique only WITHIN a source — the deterministic total order
// (and thus the cursor) must include type_str. A nil Cursor means "from the
// newest" (first page).
type Cursor struct {
	Created time.Time
	TypeStr string
	ID      int
}

// FetchFeed is the feed's keyset fetch over the materialized table: instead of a
// read-time UNION across ~18 source tables, it keyset-paginates the single feed_activity
// table (maintained by triggers — see migration 034), so per-page cost is flat
// regardless of depth / source count. `types` is the tab's activity-type set
// (nil/empty = the whole timeline). The dynamic filters that used to live in each
// UNION branch are applied once, here:
//
//   - SFW: drop is_nsfw rows (r18 website activity) for SFW viewers.
//   - resource-less: drop GALGAME_CREATION of galgames with no download resource
//     (unless showNoResource) so they never occupy a LIMIT slot.
//   - section (sectionMode): TOPIC_CREATION is split by the 资源/求助 sections
//     (g-seeking/g-other/t-help). "help" keeps only those topics, "normal" keeps
//     only the rest, "all" (or "") applies no topic split. Lets a configurable
//     tab put 普通话题 / 资源求助话题 wherever the user wants.
//
// Order + cut use the SAME (created, type, source_id) total order as the cursor
// (feed_activity has UNIQUE(type, source_id)), so serveKeyset / encode / decode
// are reused unchanged. Galgame NSFW is still dropped at enrichment (galgame-owned),
// bounded by activityMaxRounds.
func (r *ActivityRepository) FetchFeed(types []string, limit int, cur *Cursor, isSFW, showNoResource bool, sectionMode string) ([]ActivityRow, error) {
	conds := make([]string, 0, 6)
	args := make([]any, 0, 6)
	if len(types) > 0 {
		conds = append(conds, "fa.type IN ?")
		args = append(args, types)
	}
	if isSFW {
		conds = append(conds, "NOT fa.is_nsfw")
	}
	if !showNoResource {
		conds = append(conds, "(fa.type <> 'GALGAME_CREATION' OR EXISTS (SELECT 1 FROM galgame_resource r WHERE r.galgame_id = fa.galgame_id))")
	}
	const inHelp = "EXISTS (SELECT 1 FROM topic_section_relation tsr " +
		"JOIN topic_section ts ON ts.id = tsr.topic_section_id " +
		"WHERE tsr.topic_id = fa.source_id AND ts.name IN ('g-seeking','g-other','t-help'))"
	switch sectionMode {
	case "help":
		conds = append(conds, "(fa.type <> 'TOPIC_CREATION' OR "+inHelp+")")
	case "normal":
		conds = append(conds, "(fa.type <> 'TOPIC_CREATION' OR NOT "+inHelp+")")
		// "all"/"" → no topic section split (both 普通 + 资源求助 topics)
	}
	if cur != nil {
		conds = append(conds, "(fa.created, fa.type, fa.source_id) < (?, ?, ?)")
		args = append(args, cur.Created, cur.TypeStr, cur.ID)
	}

	sql := "SELECT fa.type AS type_str, fa.source_id AS id, fa.content, fa.link, fa.created, fa.user_id, fa.galgame_id FROM feed_activity fa"
	if len(conds) > 0 {
		sql += " WHERE " + strings.Join(conds, " AND ")
	}
	sql += fmt.Sprintf(" ORDER BY fa.created DESC, fa.type DESC, fa.source_id DESC LIMIT %d", limit)

	var rows []ActivityRow
	if err := r.db.Raw(sql, args...).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// FetchTopicFeed is the dedicated query for the home 话题 / 资源求助 tabs: topics as
// TOPIC_CREATION ActivityRows (so they reuse the same enrichment + cards), but
// ORDERED BY the topic's last-activity time (status_update_time, bumped by every
// reply / comment / upvote / poll) instead of the event time. Keyset on
// (status_update_time, id); the row shape matches what feed_activity emits for a
// topic (link = /topic/:id, content = title). sectionMode splits 普通 vs 资源/求助.
func (r *ActivityRepository) FetchTopicFeed(limit int, cur *Cursor, isSFW bool, sectionMode string) ([]ActivityRow, error) {
	conds := []string{"t.status != 1"} // status 1 = deleted (matches the /topic list)
	args := []any{}
	if isSFW {
		conds = append(conds, "NOT t.is_nsfw")
	}
	const inHelp = "EXISTS (SELECT 1 FROM topic_section_relation tsr " +
		"JOIN topic_section ts ON ts.id = tsr.topic_section_id " +
		"WHERE tsr.topic_id = t.id AND ts.name IN ('g-seeking','g-other','t-help'))"
	switch sectionMode {
	case "help":
		conds = append(conds, inHelp)
	case "normal":
		conds = append(conds, "NOT "+inHelp)
	}
	if cur != nil {
		conds = append(conds, "(t.status_update_time, t.id) < (?, ?)")
		args = append(args, cur.Created, cur.ID)
	}
	sql := "SELECT 'TOPIC_CREATION' AS type_str, t.id, t.title AS content, " +
		"'/topic/' || t.id::text AS link, t.status_update_time AS created, " +
		"t.user_id, 0 AS galgame_id FROM topic t WHERE " + strings.Join(conds, " AND ") +
		fmt.Sprintf(" ORDER BY t.status_update_time DESC, t.id DESC LIMIT %d", limit)
	var rows []ActivityRow
	if err := r.db.Raw(sql, args...).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// LatestActivityRow is a topic's most-recent reply OR comment (whichever is newer),
// for the feed card's 最新 line. Kind is "reply" / "comment"; ID is that row's id (a
// reply id when Kind=="reply", for deduping against the best answer / 高赞回复).
type LatestActivityRow struct {
	TopicID int       `gorm:"column:topic_id"`
	Kind    string    `gorm:"column:kind"`
	ID      int       `gorm:"column:id"`
	Floor   int       `gorm:"column:floor"`
	Content string    `gorm:"column:content"`
	UserID  int       `gorm:"column:user_id"`
	Created time.Time `gorm:"column:created"`
}

// FetchTopicLatestActivity batch-loads each topic's newest reply-or-comment in one
// query (UNION of both, DISTINCT ON the topic, newest first). Keyed by topic id.
func (r *ActivityRepository) FetchTopicLatestActivity(ids []int) (map[int]LatestActivityRow, error) {
	out := map[int]LatestActivityRow{}
	if len(ids) == 0 {
		return out, nil
	}
	var rows []LatestActivityRow
	if err := r.db.Raw(`
		SELECT DISTINCT ON (topic_id) topic_id, kind, id, floor, content, user_id, created FROM (
			SELECT topic_id, 'reply' AS kind, id, floor, SUBSTRING(content, 1, 200) AS content, user_id, created
				FROM topic_reply WHERE topic_id IN ? AND status = 0
			UNION ALL
			SELECT topic_id, 'comment' AS kind, id, 0 AS floor, SUBSTRING(content, 1, 200) AS content, user_id, created
				FROM topic_comment WHERE topic_id IN ? AND status = 0
		) x
		ORDER BY topic_id, created DESC, id DESC`, ids, ids).
		Scan(&rows).Error; err != nil {
		return out, err
	}
	for _, row := range rows {
		out[row.TopicID] = row
	}
	return out, nil
}
