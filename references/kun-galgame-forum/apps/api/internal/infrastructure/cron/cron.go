package cron

import (
	"context"
	"log/slog"
	"time"

	"kun-galgame-api/internal/infrastructure/viewstats"
	"kun-galgame-api/pkg/imageclient"

	"github.com/redis/go-redis/v9"
	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

// scheduleTZ pins all cron schedules to a fixed zone so the daily-reset /
// check-in boundary tracks the users' calendar day, not the host's local TZ
// (servers commonly run UTC). Without this, "0 0 * * *" fires at host midnight,
// shifting every user's daily window.
const scheduleTZ = "Asia/Shanghai"

// Start creates and starts all scheduled tasks. Returns a stop function.
//
// galgameClaimSync (optional, may be nil) drives the periodic ingestion of
// claim-state transitions from the registry's claim-event feed. Scheduled
// every 10 minutes so a reviewer's decision — and the +3 that rides on it —
// reaches the submitter within a normal page-refresh window.
func Start(db *gorm.DB, rdb *redis.Client, imgCli *imageclient.Client, galgameClaimSync func(), galgameRevisionSync func()) func() {
	loc, err := time.LoadLocation(scheduleTZ)
	if err != nil {
		slog.Warn("加载定时任务时区失败, 回退到进程本地时区", "tz", scheduleTZ, "error", err)
		loc = time.Local
	}
	c := cron.New(cron.WithLocation(loc))

	// Daily reset at midnight: clear daily check-in, image count, toolset upload count
	c.AddFunc("0 0 * * *", func() {
		resetDaily(db)
	})

	// Daily at midnight (same beat as the reset above): roll up the windowed view
	// stats (view_7d / view_30d) from the per-day buckets, then prune old buckets.
	// cmd/view-rollup stays as a manual one-off trigger (mirrors reference-ping).
	c.AddFunc("0 0 * * *", func() {
		if err := viewstats.RunRollup(db); err != nil {
			slog.Error("浏览量滚动统计失败", "error", err)
			return
		}
		slog.Info("浏览量滚动统计完成")
	})

	// Hourly: clean up abandoned toolset upload caches
	c.AddFunc("0 * * * *", func() {
		cleanupUploadCache(rdb)
	})

	// Daily 04:00: reference-ping every content image hash so image-gc never
	// reclaims a live content image (forum owns these — see RunReferencePing).
	// Loud skip (not silent) when the image client isn't configured, so a missed
	// env doesn't quietly leave content images on the GC clock.
	if imgCli != nil {
		c.AddFunc("0 4 * * *", func() {
			distinct, updated, err := RunReferencePing(context.Background(), db, imgCli)
			if err != nil {
				slog.Error("内容图 reference-ping 失败", "distinct", distinct, "updated", updated, "error", err)
				return
			}
			slog.Info("内容图 reference-ping 完成", "distinct_hashes", distinct, "updated", updated)
		})
	} else {
		slog.Warn("image client 未配置, 跳过内容图 reference-ping —— 内容图存在被 image-gc 回收的风险")
	}

	// Every 10 min: pull claim-state transitions and apply local side effects
	// (seed the stub on live, drop it on hidden, +3 on approval). Skipped when
	// the caller didn't wire a sync (e.g. tests).
	if galgameClaimSync != nil {
		c.AddFunc("*/10 * * * *", galgameClaimSync)
	}

	// Every 10 min: mirror editing-engine revisions into the local
	// galgame_activity timeline source. Same cadence as the message sync.
	if galgameRevisionSync != nil {
		c.AddFunc("*/10 * * * *", galgameRevisionSync)
	}

	c.Start()
	slog.Info("定时任务已启动")

	return func() {
		ctx := c.Stop()
		<-ctx.Done()
		slog.Info("定时任务已停止")
	}
}

// resetDaily resets all users' daily counters to 0 at midnight.
//
// Targets `kungal_user_state`, NOT the old `"user"` table — migration 007
// dropped the daily_* columns from the identity table and moved them to
// the per-site state table. The original cron query (`UPDATE "user" SET
// daily_* = 0`) silently errored every midnight after the migration
// landed, so users who hit their daily upload caps stayed capped
// indefinitely. See user/repository/state_repo.go ResetDailyCounters for
// the mirrored repo helper.
func resetDaily(db *gorm.DB) {
	result := db.Exec(`
		UPDATE kungal_user_state SET
			daily_check_in = 0,
			daily_image_count = 0,
			daily_toolset_upload_count = 0,
			daily_toolset_upload_bytes = 0
		WHERE daily_check_in != 0
		   OR daily_image_count != 0
		   OR daily_toolset_upload_count != 0
		   OR daily_toolset_upload_bytes != 0
	`)
	if result.Error != nil {
		slog.Error("每日重置失败", "error", result.Error)
		return
	}
	slog.Info("每日重置完成", "affected", result.RowsAffected)
}

// cleanupUploadCache removes abandoned toolset upload artifacts from Redis.
// S3 cleanup is skipped here since S3 lifecycle rules handle orphaned objects.
func cleanupUploadCache(rdb *redis.Client) {
	ctx := context.Background()
	keys, err := rdb.Keys(ctx, "toolset:upload:*").Result()
	if err != nil {
		slog.Error("扫描上传缓存失败", "error", err)
		return
	}

	if len(keys) == 0 {
		return
	}

	deleted := 0
	for _, key := range keys {
		ttl, _ := rdb.TTL(ctx, key).Result()
		// Only delete keys with no TTL (stuck) or already expired
		if ttl <= 0 {
			rdb.Del(ctx, key)
			deleted++
		}
	}

	if deleted > 0 {
		slog.Info("清理上传缓存完成", "deleted", deleted)
	}
}
