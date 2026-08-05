// Daily rollup of the windowed view stats (view_7d / view_30d) for galgame /
// topic / galgame-quiz, plus a prune of old daily buckets. The forum has no
// in-app scheduler, so this is a command an external scheduler runs once a day
// (like cmd/image-refping).
//
//	docker compose -f docker-compose.prod.yml --profile jobs run --rm tools view-rollup
package main

import (
	"log/slog"
	"os"

	"kun-galgame-api/internal/infrastructure/database"
	"kun-galgame-api/internal/infrastructure/viewstats"
	"kun-galgame-api/pkg/config"
	"kun-galgame-api/pkg/logger"

	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		slog.Error("加载配置失败", "error", err)
		os.Exit(1)
	}
	logger.Init(cfg.Server.Mode)

	db := database.NewPostgres(cfg.Database, cfg.Server.Mode)

	if err := viewstats.RunRollup(db); err != nil {
		slog.Error("浏览量滚动统计失败", "error", err)
		os.Exit(1)
	}
	slog.Info("浏览量滚动统计完成 (view_7d / view_30d 已刷新, 旧桶已清理)")
}
