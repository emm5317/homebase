package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	homebase "github.com/emm5317/homebase"
	"github.com/emm5317/homebase/internal/config"
	"github.com/emm5317/homebase/internal/engine"
	"github.com/emm5317/homebase/internal/ingest"
	"github.com/emm5317/homebase/internal/providers"
	"github.com/emm5317/homebase/internal/scheduler"
	"github.com/emm5317/homebase/internal/store"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/static"
)

var version = "dev"

func main() {
	configPath := flag.String("config", "config.yaml", "path to config.yaml")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		os.Exit(0)
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	loc, err := time.LoadLocation(cfg.Location.Timezone)
	if err != nil {
		slog.Warn("invalid timezone, using UTC", "timezone", cfg.Location.Timezone)
		loc = time.UTC
	}

	// Initialize SQLite store
	dataStore, err := store.New("data/homebase.db")
	if err != nil {
		slog.Error("failed to initialize store", "error", err)
		os.Exit(1)
	}
	defer dataStore.Close()

	// Build providers
	var pp []providers.Provider

	if cfg.OWMAPIKey != "" {
		pp = append(pp, providers.NewWeather(cfg.OWMAPIKey, cfg.Location.Lat, cfg.Location.Lon))
		slog.Info("weather provider enabled")
	} else {
		slog.Warn("OWM_API_KEY not set, weather provider disabled")
	}

	pp = append(pp, providers.NewGarbage(cfg.Garbage, loc))
	slog.Info("garbage provider enabled")

	if cfg.MetraAPIKey != "" {
		pp = append(pp, providers.NewMetra(cfg.MetraAPIKey, cfg.Metra, loc))
		slog.Info("metra provider enabled", "route", cfg.Metra.Route, "station", cfg.Metra.Station)
	} else {
		slog.Warn("METRA_API_KEY not set, metra provider disabled")
	}

	// Reminders provider — always registered; empty list → no-op.
	pp = append(pp, providers.NewReminders(cfg.Reminders, loc))
	slog.Info("reminders provider enabled", "count", len(cfg.Reminders))

	// Meals provider — only when rotation and start_date are configured.
	if len(cfg.Meals.Rotation) > 0 && cfg.Meals.StartDate != "" {
		pp = append(pp, providers.NewMeals(cfg.Meals, loc))
		slog.Info("meals provider enabled", "rotation_length", len(cfg.Meals.Rotation))
	} else {
		slog.Debug("meals disabled (no rotation configured)")
	}

	// Skylight provider is still a stub.
	// pp = append(pp, providers.NewSkylight(cfg.SkylightEmail, cfg.SkylightPassword, cfg.SkylightFrameID))

	// Start scheduler
	sched := scheduler.New(dataStore, pp...)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sched.Start(ctx)

	// Build context engine
	eng := engine.New(dataStore, loc, cfg.QuietHours)

	// Create ingest handler
	ingestHandler := ingest.NewHandler(dataStore, cfg)

	// Create Fiber app
	app := fiber.New(fiber.Config{
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		AppName:      "homebase",
	})

	// API routes
	app.Get("/api/cards", func(c fiber.Ctx) error {
		params := engine.ParseQuery(c.Query("mode"), c.Query("max"))
		cards := eng.Cards(params)
		return c.JSON(cards)
	})

	app.Get("/api/health", func(c fiber.Ctx) error {
		var mem runtime.MemStats
		runtime.ReadMemStats(&mem)

		return c.JSON(fiber.Map{
			"status":    "ok",
			"version":   version,
			"uptime":    time.Since(startTime).String(),
			"providers": sched.Statuses(),
			"memory_mb": mem.Alloc / 1024 / 1024,
		})
	})

	app.Post("/api/ingest", ingestHandler.Handle)

	// Static files — serve embedded frontend
	staticFS, err := fs.Sub(homebase.StaticFiles, "static")
	if err != nil {
		slog.Error("failed to create static FS", "error", err)
		os.Exit(1)
	}
	app.Use("/", static.New("", static.Config{
		FS: staticFS,
	}))

	// Start server
	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	slog.Info("starting homebase", "addr", addr, "version", version)

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		slog.Info("shutting down...")
		sched.Stop()
		app.Shutdown()
	}()

	if err := app.Listen(addr); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}

var startTime = time.Now()
