// One-off reference-ping of every forum content image hash (the /image/<hash>
// tokens in topic / reply / chat / comment). Runs the exact same logic as the
// daily cron (cron.RunReferencePing) — exposed as a command because forum has no
// admin job-run endpoint, so the FIRST ping (right after the migration) and any
// future manual re-ping go through the tools image.
//
// `updated` should come back ≈ the current distinct content-image hash count;
// that confirms image_service recognised them and reset their GC clock.
//
//	docker compose -f docker-compose.prod.yml --profile jobs run --rm tools image-refping
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"kun-galgame-api/internal/infrastructure/cron"
	"kun-galgame-api/internal/infrastructure/database"
	"kun-galgame-api/pkg/config"
	"kun-galgame-api/pkg/imageclient"
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

	if cfg.ImageClient.ClientID == "" || cfg.ImageClient.ClientSecret == "" {
		slog.Error("image_service 未配置 (KUN_IMAGE_CLIENT_ID / KUN_IMAGE_CLIENT_SECRET)")
		os.Exit(1)
	}
	imgCli := imageclient.New(imageclient.Config{
		BaseURL:      cfg.ImageClient.BaseURL,
		CDNBase:      cfg.NextMoeAPI.ImageCDNBase,
		ClientID:     cfg.ImageClient.ClientID,
		ClientSecret: cfg.ImageClient.ClientSecret,
	})
	db := database.NewPostgres(cfg.Database, cfg.Server.Mode)

	distinct, updated, err := cron.RunReferencePing(context.Background(), db, imgCli)
	if err != nil {
		slog.Error("内容图 reference-ping 失败", "distinct", distinct, "updated", updated, "error", err)
		os.Exit(1)
	}
	slog.Info("内容图 reference-ping 完成", "distinct_hashes", distinct, "updated", updated)
	fmt.Printf("完成: ping %d 个去重 hash, image_service updated %d。\n", distinct, updated)
}
