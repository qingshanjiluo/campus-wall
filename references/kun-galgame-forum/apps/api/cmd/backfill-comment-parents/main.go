package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Backfill parent_comment_id for legacy (pre-nesting) comments. We never recorded
// WHICH comment a reply answered — only target_user_id (the @-mentioned user).
// Heuristic: a comment's parent is the most recent comment by its target_user, on
// the same reply, posted before it. Parents are therefore always older → no cycles.
//
// Safe + reversible:
//   - touches ONLY rows where parent_comment_id IS NULL, so it won't disturb
//     comments created with the new parent-aware flow, and re-running is a no-op
//     for already-nested rows;
//   - to undo: UPDATE topic_comment SET parent_comment_id = NULL;  (then re-run).
//
//	go run ./cmd/backfill-comment-parents          # dry-run — counts only
//	go run ./cmd/backfill-comment-parents --apply  # write parent_comment_id
func main() {
	apply := flag.Bool("apply", false, "write the changes (default: dry-run, counts only)")
	flag.Parse()

	_ = godotenv.Load()
	db, err := gorm.Open(postgres.Open(os.Getenv("KUN_DATABASE_URL")),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		fmt.Println("连接数据库失败:", err)
		os.Exit(1)
	}

	// The candidate parent for each currently-flat comment (NULL if none → stays
	// top-level: it replied to the reply itself, not to another comment).
	const candidateSQL = `
		SELECT c1.id AS cid,
			(SELECT c2.id FROM topic_comment c2
			 WHERE c2.topic_reply_id = c1.topic_reply_id
			   AND c2.user_id = c1.target_user_id
			   AND c2.created < c1.created
			   AND c2.id <> c1.id
			 ORDER BY c2.created DESC LIMIT 1) AS pid
		FROM topic_comment c1
		WHERE c1.parent_comment_id IS NULL`

	var flat, wouldNest int64
	db.Raw(`SELECT count(*) FROM topic_comment WHERE parent_comment_id IS NULL`).Scan(&flat)
	db.Raw(`SELECT count(*) FROM (` + candidateSQL + `) s WHERE s.pid IS NOT NULL`).Scan(&wouldNest)
	fmt.Printf("当前顶层评论: %d, 可重建父子关系: %d\n", flat, wouldNest)

	if !*apply {
		fmt.Println("dry-run（加 --apply 实际写入），未做任何更改。")
		return
	}

	res := db.Exec(`
		UPDATE topic_comment c
		SET parent_comment_id = sub.pid
		FROM (` + candidateSQL + `) sub
		WHERE c.id = sub.cid AND sub.pid IS NOT NULL`)
	if res.Error != nil {
		fmt.Println("回填失败:", res.Error)
		os.Exit(1)
	}
	fmt.Printf("回填完成: %d 条评论已嵌套到父评论下。\n", res.RowsAffected)
}
