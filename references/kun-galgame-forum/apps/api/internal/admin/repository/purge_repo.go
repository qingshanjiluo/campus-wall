package repository

import (
	"fmt"

	"kun-galgame-api/internal/admin/dto"

	"gorm.io/gorm"
)

// PurgeRepository hard-deletes every trace a user has produced in kungal, in
// one all-or-nothing transaction, then fixes the denormalized counters on any
// surviving parent rows.
//
// Design — relies on the DB's ON DELETE CASCADE graph (verified against the
// live schema): every child→parent and *_id→user FK in kungal is ON DELETE
// CASCADE. So for OWNED ROOT entities (topic / galgame_website /
// galgame_toolset) we delete just the root by user_id and let the DB cascade
// the entire subtree — guaranteed complete, and immune to "forgot a child
// table" bugs. We do NOT delete the local `user` row (it mirrors the OAuth
// store and has no FK from kungal_user_state, so deleting it would desync
// OAuth and orphan state); that is why the user_id cascades never fire on
// their own and every table must be cleared explicitly by user_id here. The
// kungal-local state row itself IS deleted explicitly (phase F2).
//
// Counters: cascades do NOT maintain denormalized counts (no triggers), so
// after deleting the user's content on OTHER users' entities we recompute the
// affected parents' counters from the surviving rows.
//
// Scope: account identity is owned by OAuth; galgame ENTRIES are galgame-owned
// (local `galgame` has no user_id) — both out of reach by design. Admin-only
// content (doc_article / todo / update_log / unmoe, all RequireModerator()) is
// intentionally NOT handled here because the service refuses to purge any user
// with the moderation capability, so such rows can never belong to a purgeable
// user.
// For private chat we delete only the user's OWN sent messages (keeping the
// counterparty's), then repair / remove the affected rooms.
type PurgeRepository struct {
	db *gorm.DB
}

func NewPurgeRepository(db *gorm.DB) *PurgeRepository {
	return &PurgeRepository{db: db}
}

// CountUserContent returns a read-only breakdown used to preview a purge.
// NOTE: this counts the user's directly-authored rows + cast interactions; the
// actual delete also removes cascaded descendants (sub-replies, likes, links,
// reactions, …), so the real deleted row count is higher than Total.
func (r *PurgeRepository) CountUserContent(userID int) dto.UserContentStats {
	return r.counts(r.db, userID)
}

func (r *PurgeRepository) counts(q *gorm.DB, userID int) dto.UserContentStats {
	var s dto.UserContentStats
	countBy := func(table, col string) int64 {
		var n int64
		q.Table(table).Where(col+" = ?", userID).Count(&n)
		return n
	}

	s.Topics = countBy("topic", "user_id")
	s.Replies = countBy("topic_reply", "user_id")
	s.TopicComments = countBy("topic_comment", "user_id")
	s.Ratings = countBy("galgame_rating", "user_id")
	s.Resources = countBy("galgame_resource", "user_id")
	s.Websites = countBy("galgame_website", "user_id")
	s.Toolsets = countBy("galgame_toolset", "user_id")
	s.ToolsetResources = countBy("galgame_toolset_resource", "user_id")
	s.ChatMessages = countBy("chat_message", "sender_id")
	// Notifications the user generated for others + their own inbox.
	s.Messages = countBy("message", "sender_id") + countBy("message", "receiver_id")

	// The four legacy comment tables (galgame_comment / galgame_rating_comment /
	// galgame_website_comment / galgame_toolset_comment) were dropped in charter
	// step 06a — comments moved to the community primitive; their live count is
	// surfaced separately via CommunityPosts (set by the purge service).
	for _, t := range interactionTables {
		s.Interactions += countBy(t, "user_id")
	}

	s.Total = s.Topics + s.Replies + s.TopicComments +
		s.Ratings + s.Resources + s.Websites +
		s.Toolsets + s.ToolsetResources +
		s.ChatMessages + s.Messages + s.Interactions
	return s
}

// interactionTables are the user-cast interaction rows cleared in the purge
// (used for the preview count). parent-counter recompute is driven separately
// by interactionDeletes.
var interactionTables = []string{
	"topic_like", "topic_dislike", "topic_favorite", "topic_upvote",
	"topic_reply_like", "topic_reply_dislike", "topic_comment_like",
	"topic_poll_vote",
	"galgame_like", "galgame_favorite",
	// galgame_post_like = the LIVE community-comment like footprint (charter step
	// 06a); replaces the dropped frozen galgame_comment_like.
	"galgame_post_like",
	"galgame_rating_like", "galgame_resource_like",
	"galgame_website_like", "galgame_website_favorite",
	"galgame_toolset_practicality", "galgame_toolset_contributor",
}

// PurgeUserContent deletes everything authored/cast by userID and returns the
// pre-delete preview breakdown. The whole operation is one transaction: any
// error aborts and rolls back (no partial purge).
func (r *PurgeRepository) PurgeUserContent(userID int) (dto.UserContentStats, error) {
	stats := r.counts(r.db, userID)

	err := r.db.Transaction(func(tx *gorm.DB) error {
		// ── Capture surviving-parent id sets BEFORE deleting, for the
		// post-delete counter recompute (these are bounded by the number of
		// distinct parents the user touched — small, safe for an IN list). ──
		var affTopics, affReplies, affGalgames, affChatRooms []int
		captures := []struct {
			dst *[]int
			sql string
		}{
			{&affTopics, `SELECT DISTINCT topic_id FROM topic_reply WHERE user_id = ?
				UNION SELECT DISTINCT topic_id FROM topic_comment WHERE user_id = ?`},
			{&affReplies, `SELECT DISTINCT topic_reply_id FROM topic_comment WHERE user_id = ?`},
			{&affGalgames, `SELECT DISTINCT galgame_id FROM galgame_rating WHERE user_id = ?
				UNION SELECT DISTINCT galgame_id FROM galgame_resource WHERE user_id = ?`},
			// NOTE: galgame_website affected-set no longer captured — its
			// comment_count is NOT recomputed here (see phase H).
			// Rooms the user sent messages in OR merely joined — so the
			// empty-room cleanup below also removes rooms they joined but
			// never spoke in (otherwise their participant row is deleted yet
			// the room lingers).
			{&affChatRooms, `SELECT DISTINCT chat_room_id FROM chat_message WHERE sender_id = ?
				UNION SELECT chat_room_id FROM chat_room_participant WHERE user_id = ?`},
		}
		for _, c := range captures {
			// each ? in the fragment binds the same userID
			args := make([]any, countPlaceholders(c.sql))
			for i := range args {
				args[i] = userID
			}
			if err := tx.Raw(c.sql, args...).Scan(c.dst).Error; err != nil {
				return err
			}
		}

		// ── A. Owned ROOT entities — CASCADE removes their whole subtree. ──
		// topic → replies, comments, likes/dislikes/favorites/upvotes, polls
		//   (+options+votes), tag/section relations.
		// galgame_website → likes, favorites, tag relations.
		// galgame_toolset → resources, contributors, practicality, aliases,
		//   category relations.
		// (website / toolset comments moved to the community primitive in charter
		//   step 06a and are purged via the community AuthorPurge, not by cascade.)
		// topic_poll WHERE user_id: polls the user created on OTHER users'
		//   topics (own-topic polls already went with the topic above).
		for _, q := range []string{
			"DELETE FROM topic WHERE user_id = ?",
			"DELETE FROM galgame_website WHERE user_id = ?",
			"DELETE FROM galgame_toolset WHERE user_id = ?",
			"DELETE FROM topic_poll WHERE user_id = ?",
		} {
			if err := del(tx, q, userID); err != nil {
				return err
			}
		}

		// ── B. The user's content on OTHER users' entities. Each row's own
		// children (likes, targets, sub-replies, links, providers) cascade. The
		// four legacy comment tables are gone (charter step 06a; migration 060) —
		// community comments are purged separately via the community AuthorPurge. ──
		for _, q := range []string{
			"DELETE FROM topic_reply WHERE user_id = ?", // SET NULL clears any topic best_answer/pin pointing here
			"DELETE FROM topic_comment WHERE user_id = ?",
			"DELETE FROM galgame_rating WHERE user_id = ?",
			"DELETE FROM galgame_resource WHERE user_id = ?",
			"DELETE FROM galgame_toolset_resource WHERE user_id = ?",
		} {
			if err := del(tx, q, userID); err != nil {
				return err
			}
		}

		// ── C. Interactions cast by the user, recomputing each parent count. ──
		for _, it := range interactionDeletes {
			var ids []int
			if it.parentTable != "" {
				if err := tx.Raw(
					"SELECT DISTINCT "+it.parentCol+" FROM "+it.table+" WHERE user_id = ?", userID,
				).Scan(&ids).Error; err != nil {
					return err
				}
			}
			if err := del(tx, "DELETE FROM "+it.table+" WHERE user_id = ?", userID); err != nil {
				return err
			}
			if it.parentTable != "" {
				if err := recount(tx, it.parentTable, it.countCol, it.table, it.parentCol, ids); err != nil {
					return err
				}
			}
		}

		// ── D. Private chat: only the user's OWN sent messages (the
		// counterparty's are kept). Deleting them cascades to reactions /
		// read receipts ON those messages; we separately clear the user's
		// reactions / reads / membership on OTHER messages & rooms. ──
		for _, q := range []string{
			"DELETE FROM chat_message WHERE sender_id = ?",
			"DELETE FROM chat_message_reaction WHERE user_id = ?",
			"DELETE FROM chat_message_read_by WHERE user_id = ?",
			"DELETE FROM chat_room_admin WHERE user_id = ?",
			"DELETE FROM chat_room_participant WHERE user_id = ?",
		} {
			if err := del(tx, q, userID); err != nil {
				return err
			}
		}
		if len(affChatRooms) > 0 {
			// Repair the denormalized last-message snapshot from the latest
			// surviving message (sender_name can't be recomputed in SQL — the
			// name lives in OAuth — so blank it; the FE hydrates by id).
			if err := del(tx, `UPDATE chat_room cr SET
					last_message_content = lm.content,
					last_message_time = lm.created,
					last_message_sender_id = lm.sender_id,
					last_message_sender_name = ''
				FROM (
					SELECT DISTINCT ON (chat_room_id) chat_room_id, content, created, sender_id
					FROM chat_message WHERE chat_room_id IN ?
					ORDER BY chat_room_id, created DESC
				) lm
				WHERE cr.id = lm.chat_room_id`, affChatRooms); err != nil {
				return err
			}
			// Rooms left with no messages at all (spam-only DMs) are removed
			// entirely; CASCADE clears their participants / admins.
			if err := del(tx, `DELETE FROM chat_room WHERE id IN ?
				AND NOT EXISTS (SELECT 1 FROM chat_message m WHERE m.chat_room_id = chat_room.id)`,
				affChatRooms); err != nil {
				return err
			}
		}

		// ── E. Notification inbox + system messages + read cursors. The
		// `message` table is system-generated notifications (not free DMs);
		// clear both the ones the user's actions generated for others
		// (sender_id) and the user's own inbox (receiver_id) — this one has
		// TWO placeholders, the rest have one, so they can't share a loop. ──
		if err := del(tx, "DELETE FROM message WHERE sender_id = ? OR receiver_id = ?", userID, userID); err != nil {
			return err
		}
		for _, q := range []string{
			"DELETE FROM system_message WHERE user_id = ?",
			"DELETE FROM system_message_read_state WHERE user_id = ?",
			"DELETE FROM wiki_message_read_state WHERE user_id = ?",
		} {
			if err := del(tx, q, userID); err != nil {
				return err
			}
		}

		// ── F. Social graph, both directions. ──
		for _, q := range []string{
			"DELETE FROM user_follow WHERE follower_id = ? OR followed_id = ?",
			"DELETE FROM user_friend WHERE user_id = ? OR friend_id = ?",
		} {
			if err := del(tx, q, userID, userID); err != nil {
				return err
			}
		}

		// ── F2. The user's own kungal-local account state (moemoepoint
		// balance, check-in / daily counters). It has NO FK, so nothing
		// cascades it — it must be deleted explicitly to truly erase the
		// user. Safe: StateRepository.Ensure lazily recreates a default row
		// if a (non-banned) user ever returns. ──
		if err := del(tx, "DELETE FROM kungal_user_state WHERE user_id = ?", userID); err != nil {
			return err
		}

		// ── G. Recompute counters on SURVIVING parents whose children were
		// removed in phase B above (interaction counters handled in C). ──
		recounts := []struct {
			parentTable, countCol, childTable, parentCol string
			ids                                          []int
		}{
			{"topic", "reply_count", "topic_reply", "topic_id", affTopics},
			{"topic", "comment_count", "topic_comment", "topic_id", affTopics},
			{"topic_reply", "comment_count", "topic_comment", "topic_reply_id", affReplies},
			{"galgame", "rating_count", "galgame_rating", "galgame_id", affGalgames},
			{"galgame", "resource_count", "galgame_resource", "galgame_id", affGalgames},
			// The four legacy comment tables are gone (migration 060), so the
			// galgame_rating.comment_count recompute (and the galgame_comment
			// like_count recompute, via the removed galgame_comment_like interaction)
			// are retired. galgame.comment_count / galgame_website.comment_count stay
			// LIVE display counters maintained ±1 by the community BFF (ruling 11) —
			// never recomputed here, so the purge can't clobber them with a stale value.
		}
		for _, rc := range recounts {
			if err := recount(tx, rc.parentTable, rc.countCol, rc.childTable, rc.parentCol, rc.ids); err != nil {
				return err
			}
		}

		return nil
	})

	return stats, err
}

// interactionDelete: an interaction table the user casts rows into, plus the
// parent counter to recompute after removal. parentTable == "" → no counter.
type interactionDelete struct {
	table       string
	parentCol   string
	parentTable string
	countCol    string
}

var interactionDeletes = []interactionDelete{
	{"topic_like", "topic_id", "topic", "like_count"},
	{"topic_dislike", "topic_id", "topic", "dislike_count"},
	{"topic_favorite", "topic_id", "topic", "favorite_count"},
	{"topic_upvote", "topic_id", "topic", "upvote_count"},
	{"topic_reply_like", "topic_reply_id", "topic_reply", "like_count"},
	{"topic_reply_dislike", "topic_reply_id", "topic_reply", "dislike_count"},
	{"topic_comment_like", "topic_comment_id", "", ""},
	{"topic_poll_vote", "option_id", "topic_poll_option", "vote_count"},
	{"galgame_like", "galgame_id", "galgame", "like_count"},
	{"galgame_favorite", "galgame_id", "galgame", "favorite_count"},
	// galgame_post_like: the purged user's OWN like footprint on community comments
	// (charter step 06a — mirrors infra's reactions_deleted). No parent counter to
	// recompute (the display count is derived live), so no counter clobber. Likes
	// RECEIVED on their tombstoned posts stay, matching infra's GDPR reading.
	{"galgame_post_like", "post_id", "", ""},
	{"galgame_rating_like", "galgame_rating_id", "galgame_rating", "like_count"},
	{"galgame_resource_like", "galgame_resource_id", "galgame_resource", "like_count"},
	{"galgame_website_like", "website_id", "galgame_website", "like_count"},
	{"galgame_website_favorite", "website_id", "galgame_website", "favorite_count"},
	{"galgame_toolset_practicality", "toolset_id", "", ""},
	{"galgame_toolset_contributor", "toolset_id", "", ""},
}

// del runs a DELETE/UPDATE and PROPAGATES the error (unlike a fire-and-forget
// Exec) so any mid-transaction failure aborts and rolls back the whole purge.
func del(tx *gorm.DB, query string, args ...any) error {
	return tx.Exec(query, args...).Error
}

// recount sets parentTable.countCol = COUNT(child rows) for the given surviving
// parent ids. Identifiers are caller-owned constants — never user input — so
// the fmt.Sprintf is injection-safe; values bind as parameters.
func recount(tx *gorm.DB, parentTable, countCol, childTable, parentCol string, ids []int) error {
	if len(ids) == 0 {
		return nil
	}
	sql := fmt.Sprintf(
		"UPDATE %s SET %s = (SELECT COUNT(*) FROM %s WHERE %s.%s = %s.id) WHERE %s.id IN ?",
		parentTable, countCol, childTable, childTable, parentCol, parentTable, parentTable,
	)
	return tx.Exec(sql, ids).Error
}

// countPlaceholders counts '?' bind markers in a SQL fragment so a single
// repeated userID can be expanded to the right arity.
func countPlaceholders(sql string) int {
	n := 0
	for _, c := range sql {
		if c == '?' {
			n++
		}
	}
	return n
}
