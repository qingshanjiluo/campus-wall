// cmd/sync-moemoepoint is a ONE-TIME (re-runnable) cache seeder for the
// moemoepoint OAuth cutover.
//
// After OAuth becomes the unified source of truth and the §6 merge sets each
// user's unified starting balance, kungal's local kungal_user_state.moemoepoint
// column (now just a read-cache for ranking / profile / /auth/me) is stale.
// This pulls every local user's authoritative balance from OAuth
// (GET /users/:id/moemoepoint) and writes it into the cache, so the ranking
// immediately reflects the unified balance instead of waiting for each user's
// next earn (which is what mirrors the balance going forward).
//
// Idempotent + safe to re-run. Read-only against OAuth; only UPDATEs the local
// cache. Users OAuth doesn't know yet are logged and skipped (their stale local
// value is left untouched). Mirrors kun-galgame-patch's equivalent.
//
// Usage:
//
//	go run ./cmd/sync-moemoepoint                 # seed all users
//	go run ./cmd/sync-moemoepoint -dry-run        # print, don't write
//	go run ./cmd/sync-moemoepoint -concurrency=16
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"kun-galgame-api/pkg/userclient"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	_ = godotenv.Load()

	dryRun := flag.Bool("dry-run", false, "只打印将写入的值，不更新本地缓存")
	concurrency := flag.Int("concurrency", 8, "并发拉取 OAuth 余额的数量")
	flag.Parse()

	db, err := gorm.Open(postgres.Open(os.Getenv("KUN_DATABASE_URL")),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		slog.Error("数据库连接失败", "error", err)
		os.Exit(1)
	}

	uc := userclient.New(userclient.Config{
		BaseURL:      os.Getenv("OAUTH_SERVER_URL"),
		ClientID:     os.Getenv("OAUTH_CLIENT_ID"),
		ClientSecret: os.Getenv("OAUTH_CLIENT_SECRET"),
	})

	var ids []int
	if err := db.Table("kungal_user_state").Order("user_id").Pluck("user_id", &ids).Error; err != nil {
		slog.Error("拉取本地用户 id 失败", "error", err)
		os.Exit(1)
	}
	fmt.Printf("本地用户数：%d（并发 %d，dry-run=%v）\n", len(ids), *concurrency, *dryRun)

	var synced, failed, unchanged int64
	sem := make(chan struct{}, *concurrency)
	var wg sync.WaitGroup

	for _, id := range ids {
		wg.Add(1)
		sem <- struct{}{}
		go func(uid int) {
			defer wg.Done()
			defer func() { <-sem }()

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			bal, err := uc.GetMoemoepoint(ctx, uid)
			if err != nil {
				atomic.AddInt64(&failed, 1)
				slog.Warn("读取 OAuth 余额失败（跳过）", "user_id", uid, "error", err)
				return
			}
			if *dryRun {
				atomic.AddInt64(&synced, 1)
				return
			}
			res := db.Exec(`UPDATE kungal_user_state SET moemoepoint = ? WHERE user_id = ?`, bal, uid)
			if res.Error != nil {
				atomic.AddInt64(&failed, 1)
				slog.Warn("写入本地缓存失败", "user_id", uid, "balance", bal, "error", res.Error)
				return
			}
			if res.RowsAffected == 0 {
				atomic.AddInt64(&unchanged, 1)
				return
			}
			atomic.AddInt64(&synced, 1)
		}(id)
	}
	wg.Wait()

	fmt.Printf("✅ 完成：同步 %d，未变 %d，失败/跳过 %d\n", synced, unchanged, failed)
}
