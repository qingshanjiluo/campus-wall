package database

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"strings"
	"time"

	"kun-galgame-api/pkg/config"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// withTimeZone pins the Postgres session TimeZone to Asia/Shanghai when the DSN
// doesn't already set one. Calendar-day logic (admin daily-stats date_trunc,
// the midnight reset cron) must agree on the day boundary; without a pinned
// zone, date_trunc('day', ...) buckets in the server's local zone (often UTC)
// while the Go side truncates in Asia/Shanghai → off-by-one oldest bucket.
func withTimeZone(dsn string) string {
	if strings.Contains(dsn, "TimeZone=") || strings.Contains(dsn, "timezone=") {
		return dsn
	}
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	return dsn + sep + "TimeZone=Asia/Shanghai"
}

func NewPostgres(cfg config.DatabaseConfig, mode string) *gorm.DB {
	logLevel := logger.Warn
	if mode == "dev" {
		logLevel = logger.Info
	}

	// IgnoreRecordNotFoundError: ErrRecordNotFound is normal control flow here,
	// not a failure — GalgameRepository.FindLocal probes for a lazily-created
	// local stats stub (a galgame only viewed, never interacted with, has none),
	// and ToggleLike/ToggleFavorite use First() to test for an existing
	// like/favorite before creating one. The default GORM logger reports every
	// such miss at error level, flooding prod logs with harmless
	// "record not found" lines. Silence those while keeping the 200ms slow-query
	// trace (which still surfaces e.g. the activity-feed UNION query).
	gormLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             200 * time.Millisecond,
			LogLevel:                  logLevel,
			IgnoreRecordNotFoundError: true,
			Colorful:                  true,
		},
	)

	db, err := gorm.Open(postgres.Open(withTimeZone(cfg.URL)), &gorm.Config{
		Logger:                 gormLogger,
		SkipDefaultTransaction: true,
	})
	if err != nil {
		panic(fmt.Sprintf("连接数据库失败: %v", err))
	}

	sqlDB, err := db.DB()
	if err != nil {
		panic(fmt.Sprintf("获取数据库连接池失败: %v", err))
	}

	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetime) * time.Second)

	slog.Info("数据库连接成功")
	return db
}
