// Command isobuilder serves the ISO build platform: it inventories base images,
// manages projects, runs the repack pipeline and publishes finished images to
// object storage.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"isobuilder/internal/builder"
	"isobuilder/internal/config"
	"isobuilder/internal/httpapi"
	"isobuilder/internal/settings"
	"isobuilder/internal/storage"
	"isobuilder/internal/store"
	"isobuilder/internal/version"
)

func main() {
	configPath := flag.String("c", "", "YAML 配置文件路径（可设 database_url，覆盖环境变量）")
	showVersion := flag.Bool("v", false, "显示版本信息并退出")
	flag.Parse()

	if *showVersion {
		fmt.Println(version.Full())
		os.Exit(0)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Error("加载配置失败", "err", err)
		os.Exit(1)
	}

	var st *store.Store
	if cfg.DatabaseURL != "" {
		st, err = store.OpenDatabase(cfg.DatabaseURL)
	} else {
		st, err = store.OpenSQLite(cfg.DBPath)
	}
	if err != nil {
		logger.Error("打开数据库失败", "err", err)
		os.Exit(1)
	}
	defer st.Close()

	// ISO paths used to be stored absolute, which orphaned every row whenever
	// data_dir moved. Rewrite them once at startup; a no-op after the first run.
	if n, err := st.NormalizeISOPaths(cfg.DataDir); err != nil {
		logger.Warn("规范化镜像路径失败", "err", err)
	} else if n > 0 {
		logger.Info("已将镜像路径改为相对于 data_dir", "count", n)
	}

	// The artifact backend is resolved from the database first, so settings
	// edited in the UI survive a restart; the environment provides the default.
	// The switcher lets a settings change take effect without a restart.
	effective := settings.Load(st, cfg.Storage)
	cfg.Storage = effective.ToConfig()
	if err := effective.Validate(); err != nil {
		logger.Warn("对象存储配置不可用，产物将留在本机", "err", err)
	}
	blob := storage.NewSwitcher(storage.New(cfg.Storage))
	manager := builder.NewManager(cfg, st, blob)

	api := httpapi.New(cfg, st, manager, blob, logger)
	api.RegisterBuiltinKickstarts()
	api.ResummarizeProjects()

	// A queued build cannot survive a restart: its process is gone. The store
	// marks those rows failed on open, so nothing is silently stuck.
	if ids, err := st.ActiveBuildIDs(); err == nil && len(ids) > 0 {
		logger.Warn("中断的构建已标记为失败", "count", len(ids))
	}

	server := &http.Server{
		Addr:              cfg.Addr,
		Handler:           api.Handler(),
		ReadHeaderTimeout: 30 * time.Second,
		// No write timeout: the log endpoint streams for the whole build and
		// downloads of multi-gigabyte ISOs must not be cut off.
		IdleTimeout: 120 * time.Second,
	}

	logger.Info("isobuilder 启动",
		"addr", cfg.Addr,
		"config_file", *configPath,
		"data_dir", cfg.DataDir,
		"database", st.Dialect(),
		"storage", blob.Kind(),
		"bucket", blob.Bucket(),
		"storage_source", effective.Source,
		"max_concurrent_builds", cfg.MaxConcurrentBuilds,
		"auth", cfg.AuthToken != "",
	)

	done := make(chan struct{})
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		logger.Info("收到退出信号，正在停止构建并关闭服务")
		manager.CancelAll()
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
		close(done)
	}()

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("HTTP 服务退出", "err", err)
		os.Exit(1)
	}
	<-done
	manager.Wait()
	logger.Info("已退出")
}
