// Backfill topic.cover_images from the images already embedded in each topic's
// body: take the first N distinct /image/<hash> content tokens (in order of
// appearance) and store them as the topic's feed-card covers.
//
// Covers are stored as a JSON array of /image/<hash> tokens in the scalar text
// column topic.cover_images (see migration 029 + model.ImageTokens for why
// tokens-in-text, not text[]). This tool writes exactly that shape, so the
// daily reference-ping keeps the harvested covers alive automatically.
//
// SCOPE: only /image/<hash> CONTENT TOKENS are harvested. Topics whose body
// images are arbitrary external URLs (topics — unlike chat — allow any host)
// are left without an auto-cover; their author can add covers by hand. Only
// topics whose cover_images is still ” (unset) are touched, so this is
// idempotent and never overwrites a cover someone picked manually — a re-run
// only fills topics that gained a token-image since the last run.
//
// SAFE BY DEFAULT: -dry-run defaults to TRUE — it reports how many topics would
// get covers (and the per-count distribution), with NO DB writes. Pass
// -dry-run=false to apply. Run the 029 migration FIRST (the column must exist).
//
//	docker compose -f docker-compose.prod.yml --profile jobs run --rm tools \
//	  backfill-topic-covers                      # dry-run: report workload
//	  backfill-topic-covers -dry-run=false       # apply (first 3 token-images per topic)
//	  backfill-topic-covers -dry-run=false -max=1 # only the first image as a single cover
//	  backfill-topic-covers -dry-run=false -limit=20 # smoke-test 20 topics first
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"regexp"

	"kun-galgame-api/internal/infrastructure/database"
	"kun-galgame-api/pkg/config"
	"kun-galgame-api/pkg/logger"

	"github.com/joho/godotenv"
)

// contentImageTokenRe matches a /image/<64hex> content token — the same shape
// cron/reference_ping.go scans for. We harvest the full token (with the
// /image/ prefix) so what we store is render-ready and ping-visible.
var contentImageTokenRe = regexp.MustCompile(`/image/[0-9a-f]{64}`)

func main() {
	_ = godotenv.Load()

	dryRun := flag.Bool("dry-run", true, "TRUE (default): report workload only, no DB writes. Pass -dry-run=false to apply.")
	maxCovers := flag.Int("max", 3, "Max covers to take per topic (the first N distinct token-images in body order; capped at 9)")
	limit := flag.Int("limit", 0, "Max topics to process (0 = all); for smoke-testing -dry-run=false on a small batch")
	flag.Parse()

	if *maxCovers < 1 || *maxCovers > 9 {
		slog.Error("max 必须在 1..9 之间", "max", *maxCovers)
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		slog.Error("加载配置失败", "error", err)
		os.Exit(1)
	}
	logger.Init(cfg.Server.Mode)

	db := database.NewPostgres(cfg.Database, cfg.Server.Mode)

	type row struct {
		ID      int64
		Content string
	}
	var rows []row
	q := db.Table("topic").
		Select("id, content").
		// Only topics without covers yet, that contain at least one token image.
		Where("cover_images = ''").
		Where("content LIKE ?", "%/image/%").
		Order("id ASC")
	if *limit > 0 {
		q = q.Limit(*limit)
	}
	if err := q.Scan(&rows).Error; err != nil {
		slog.Error("扫描话题失败", "error", err)
		os.Exit(1)
	}

	slog.Info("开始回填话题封面", "dry_run", *dryRun, "max", *maxCovers, "limit", *limit, "候选话题数", len(rows))

	var wouldFill, filled int
	countDist := map[int]int{} // #covers -> #topics

	for _, r := range rows {
		covers := firstDistinctTokens(r.Content, *maxCovers)
		if len(covers) == 0 {
			continue // only external-URL images, no token to harvest
		}
		wouldFill++
		countDist[len(covers)]++

		if *dryRun {
			continue
		}

		payload, merr := json.Marshal(covers)
		if merr != nil {
			slog.Error("序列化封面失败, 跳过", "topic_id", r.ID, "error", merr)
			continue
		}
		if err := db.Exec(
			"UPDATE topic SET cover_images = ? WHERE id = ?",
			string(payload), r.ID,
		).Error; err != nil {
			slog.Error("更新话题封面失败", "topic_id", r.ID, "error", err)
			continue
		}
		filled++
		slog.Info("已回填封面", "topic_id", r.ID, "封面数", len(covers))
	}

	if *dryRun {
		fmt.Printf("dry-run 完成。可回填话题 %d / 候选 %d。各封面数分布: ", wouldFill, len(rows))
		for n := 1; n <= *maxCovers; n++ {
			if countDist[n] > 0 {
				fmt.Printf("%d张=%d ", n, countDist[n])
			}
		}
		fmt.Printf("\n加 -dry-run=false 执行。\n")
		return
	}

	slog.Info("回填完成", "已回填话题数", filled, "可回填", wouldFill, "候选", len(rows))
	fmt.Printf("回填完成: 已为 %d 个话题写入封面 (候选 %d)。\n", filled, len(rows))
}

// firstDistinctTokens returns the first `max` distinct /image/<hash> tokens in
// the order they appear in content (a repeated image counts once).
func firstDistinctTokens(content string, max int) []string {
	matches := contentImageTokenRe.FindAllString(content, -1)
	seen := make(map[string]struct{}, len(matches))
	out := make([]string, 0, max)
	for _, tk := range matches {
		if _, dup := seen[tk]; dup {
			continue
		}
		seen[tk] = struct{}{}
		out = append(out, tk)
		if len(out) >= max {
			break
		}
	}
	return out
}
