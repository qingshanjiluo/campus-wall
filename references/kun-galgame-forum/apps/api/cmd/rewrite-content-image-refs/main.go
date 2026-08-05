// Rewrite already-migrated ABSOLUTE image_service URLs in user content to the
// domain-independent `/image/<hash>` token.
//
// Context: backfill-content-images first ran writing the absolute CDN URL
// (https://<cdn>/<aa>/<bb>/<hash>.webp) into content — which re-hardcodes a
// domain into every row, the exact failure mode the image_service contract
// kills. This one-time, string-only pass converts those URLs to `/image/<hash>`,
// resolved to the CDN at render time (markdown.resolveContentImageRef) and by
// the /image/:hash 302 fallback.
//
// SAFE BY DEFAULT: -dry-run defaults TRUE (report only). Only `content` is
// written (never `updated`, so posts aren't bounced to "recently updated").
// Idempotent: after the rewrite a row no longer matches the absolute base, so a
// re-run skips it.
//
// ORDERING (critical): deploy the resolver FIRST — the Go markdown render
// resolution, the web /image/:hash 302 route, and uploads returning
// `/image/<hash>`. Only AFTER that is live, run this with -dry-run=false.
// Running it before the resolver ships would break every content image.
//
//	docker compose -f docker-compose.prod.yml --profile jobs run --rm tools \
//	  rewrite-content-image-refs                  # dry-run: count rows to rewrite
//	  rewrite-content-image-refs -dry-run=false   # apply
//	  rewrite-content-image-refs -base=https://other-cdn   # override the matched base
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"

	"kun-galgame-api/internal/infrastructure/database"
	"kun-galgame-api/pkg/config"
	"kun-galgame-api/pkg/logger"

	"github.com/joho/godotenv"
)

type target struct{ table, col string }

var targets = []target{
	{"topic", "content"},
	{"topic_reply", "content"},
	{"chat_message", "content"},
}

func main() {
	_ = godotenv.Load()

	dryRun := flag.Bool("dry-run", true, "TRUE (default): scan + report only. Pass -dry-run=false to apply.")
	baseFlag := flag.String("base", "", "Absolute CDN base to rewrite (default = cfg.NextMoeAPI.ImageCDNBase)")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		slog.Error("加载配置失败", "error", err)
		os.Exit(1)
	}
	logger.Init(cfg.Server.Mode)

	base := strings.TrimRight(orDefault(*baseFlag, cfg.NextMoeAPI.ImageCDNBase), "/")
	if base == "" {
		slog.Error("CDN base 为空 (设 KUN_IMAGE_PUBLIC_BASE_URL 或 -base)")
		os.Exit(1)
	}

	// {base}/<aa>/<bb>/<hash>[_variant].webp  →  /image/<hash>[_variant]
	re := regexp.MustCompile(
		regexp.QuoteMeta(base) + `/[0-9a-f]{2}/[0-9a-f]{2}/([0-9a-f]{64})(_[a-z0-9]+)?\.webp`,
	)

	db := database.NewPostgres(cfg.Database, cfg.Server.Mode)

	var rowsUpdated, replacements int
	slog.Info("开始把绝对 image_service URL 改写为 /image/<hash>", "dry_run", *dryRun, "base", base)

	for _, t := range targets {
		type row struct {
			ID      int64
			Content string
		}
		var rows []row
		if err := db.Table(t.table).
			Select("id, "+t.col+" AS content").
			Where(t.col+" LIKE ?", "%"+base+"%").
			Order("id ASC").
			Scan(&rows).Error; err != nil {
			slog.Error("扫描失败", "table", t.table, "error", err)
			os.Exit(1)
		}
		slog.Info("扫描完成", "table", t.table, "含绝对URL行数", len(rows))

		for _, r := range rows {
			n := len(re.FindAllString(r.Content, -1))
			if n == 0 {
				continue
			}
			if *dryRun {
				rowsUpdated++
				replacements += n
				continue
			}
			newContent := re.ReplaceAllString(r.Content, "/image/${1}${2}")
			if newContent == r.Content {
				continue
			}
			if err := db.Exec(
				"UPDATE "+t.table+" SET "+t.col+" = ? WHERE id = ?",
				newContent, r.ID,
			).Error; err != nil {
				slog.Error("更新行失败", "table", t.table, "id", r.ID, "error", err)
				continue
			}
			rowsUpdated++
			replacements += n
			slog.Info("已改写", "table", t.table, "id", r.ID, "替换处数", n)
		}
	}

	if *dryRun {
		fmt.Printf("dry-run 完成: 将改写 %d 行(%d 处绝对 URL → /image/<hash>)。加 -dry-run=false 执行。\n",
			rowsUpdated, replacements)
		return
	}
	slog.Info("改写完成", "改写行数", rowsUpdated, "替换处数", replacements)
	fmt.Printf("完成: 改写 %d 行(%d 处)。\n", rowsUpdated, replacements)
}

func orDefault(v, def string) string {
	if v != "" {
		return v
	}
	return def
}
