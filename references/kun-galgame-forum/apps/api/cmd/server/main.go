package main

import (
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"kun-galgame-api/internal/app"
	"kun-galgame-api/pkg/config"
	"kun-galgame-api/pkg/health"
	"kun-galgame-api/pkg/logger"

	"github.com/joho/godotenv"
)

func main() {
	// Load .env (ignore error in production where env vars are set externally)
	_ = godotenv.Load()

	// `server healthcheck` — used by the container HEALTHCHECK on the
	// shell-less distroless image. Probes the already-running server's
	// /healthz and exits 0/1; no-op for the normal `server` invocation.
	// Runs BEFORE config.Load() on purpose: a liveness probe needs only the
	// port, not a fully valid DB/OAuth/JWT config (which the healthcheck
	// process would otherwise have to satisfy just to GET localhost/healthz).
	serverPort := os.Getenv("SERVER_PORT")
	if serverPort == "" {
		serverPort = "2334" // keep in sync with config.envOrDefault("SERVER_PORT", ...)
	}
	port, _ := strconv.Atoi(serverPort)
	health.MaybeProbe(port, "/healthz")

	cfg, err := config.Load()
	if err != nil {
		slog.Error("加载配置失败", "error", err)
		os.Exit(1)
	}

	logger.Init(cfg.Server.Mode)

	application := app.New(cfg)

	// Graceful shutdown
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		slog.Info("正在关闭服务器...")
		_ = application.Fiber.Shutdown()
	}()

	addr := ":" + cfg.Server.Port
	slog.Info("服务器启动", "addr", addr)
	if err := application.Fiber.Listen(addr); err != nil {
		slog.Error("服务器启动失败", "error", err)
		os.Exit(1)
	}
}
