// ClawDaemon — multi-CLI AI agent orchestrator daemon.
// Entry point: wires all packages and starts the HTTP server.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yourusername/clawdaemon/internal/api"
	"github.com/yourusername/clawdaemon/internal/auth"
	"github.com/yourusername/clawdaemon/internal/character"
	"github.com/yourusername/clawdaemon/internal/cli"
	"github.com/yourusername/clawdaemon/internal/config"
	"github.com/yourusername/clawdaemon/internal/db"
	"github.com/yourusername/clawdaemon/internal/notify"
	"github.com/yourusername/clawdaemon/internal/platform"
	"github.com/yourusername/clawdaemon/internal/queue"
	"github.com/yourusername/clawdaemon/internal/scheduler"
	"github.com/yourusername/clawdaemon/internal/telegram"
	"github.com/yourusername/clawdaemon/internal/tokenizer"
	"github.com/yourusername/clawdaemon/internal/webhook"
	"github.com/yourusername/clawdaemon/internal/wizard"
	"github.com/yourusername/clawdaemon/internal/worker"
	"github.com/yourusername/clawdaemon/internal/ws"
	"github.com/yourusername/clawdaemon/web"
)

// Version is set via -ldflags at build time.
var Version = "dev"

func main() {
	// ── 0. Setup wizard — run and exit if requested ───────────────────────────
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "setup", "--setup", "-setup":
			if err := wizard.Run(Version); err != nil {
				log.Fatalf("Setup: %v", err)
			}
			os.Exit(0)
		}
	}

	log.Printf("ClawDaemon %s starting…", Version)

	// ── 1. Load configuration ────────────────────────────────────────────────
	cfg := config.Load()
	log.Printf("Config: port=%s workDir=%s", cfg.Port, cfg.WorkDir)

	// Zero-config first run: warn when no .env is present.
	if _, err := os.Stat(".env"); os.IsNotExist(err) {
		log.Println("⚠  No .env found — using built-in defaults (admin / admin, port 8080)")
		log.Println("   Run 'clawdaemon setup' to configure before going to production.")
	}

	// ── 2. Ensure work directories exist ────────────────────────────────────
	for _, dir := range []string{
		cfg.WorkDir,
		cfg.CharacterDir,
		cfg.ScreenshotsDir,
	} {
		if err := platform.EnsureDir(dir); err != nil {
			log.Fatalf("EnsureDir %s: %v", dir, err)
		}
	}

	// ── 3. Open database + migrate ───────────────────────────────────────────
	database, err := db.New(cfg.DBPath)
	if err != nil {
		log.Fatalf("db.New: %v", err)
	}
	defer database.Close()

	if err := database.Migrate(); err != nil {
		log.Fatalf("db.Migrate: %v", err)
	}
	log.Printf("Database ready: %s", cfg.DBPath)
	wizard.PrintDashboardURLs(cfg.Port)

	// Root context — cancelled on shutdown signal.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ── 4. Seed default admin user ───────────────────────────────────────────
	if err := auth.SeedAdmin(ctx, database, cfg.AdminUsername, cfg.AdminPassword); err != nil {
		log.Fatalf("SeedAdmin: %v", err)
	}

	// ── 5. WebSocket hub ─────────────────────────────────────────────────────
	hub := ws.NewHub()
	go hub.Run(ctx)

	// ── 6. Telegram bot ──────────────────────────────────────────────────────
	cmdHandler := telegram.NewCommandHandler(database)
	bot, err := telegram.New(cfg.TelegramToken, cfg.TelegramChatID, cmdHandler)
	if err != nil {
		log.Printf("Telegram init error (continuing without Telegram): %v", err)
	}
	if bot != nil {
		go bot.Start(ctx)
		log.Printf("Telegram bot started (chatID=%d)", cfg.TelegramChatID)
	}

	// ── 7. Notify + Webhook dispatchers ─────────────────────────────────────
	webhookDispatcher := webhook.New(database)
	notifier := notify.New(telegramSender(bot), webhookDispatcher)

	// ── 8. Token governor ────────────────────────────────────────────────────
	governor := tokenizer.NewGovernor(database, notifier)

	// ── 9. Character system ──────────────────────────────────────────────────
	loader := character.NewLoader(cfg.CharacterDir)
	injector := character.NewInjector(loader)

	// ── 10. Task queue ───────────────────────────────────────────────────────
	taskQueue := queue.New(database)

	// ── 11. CLI registry ─────────────────────────────────────────────────────
	registry := cli.DefaultRegistry()

	// ── 12. Worker pool ──────────────────────────────────────────────────────
	pool := worker.NewPool(database, taskQueue, registry, injector, governor, hub, notifier)
	if err := pool.LoadWorkers(ctx); err != nil {
		log.Printf("LoadWorkers: %v (no workers yet — add them via API or Telegram)", err)
	}

	// ── 13. Cron scheduler ───────────────────────────────────────────────────
	schedEngine := scheduler.New(database, taskQueue)
	if err := schedEngine.Start(ctx); err != nil {
		log.Printf("scheduler.Start: %v", err)
	}

	// ── 14. HTTP router ──────────────────────────────────────────────────────
	mux := http.NewServeMux()

	// API routes.
	api.SetupRoutes(mux, &api.Deps{
		DB:        database,
		Config:    cfg,
		Queue:     taskQueue,
		Pool:      pool,
		Hub:       hub,
		Notify:    notifier,
		Webhook:   webhookDispatcher,
		Scheduler: schedEngine,
		Loader:    loader,
		Injector:  injector,
	})

	// WebSocket endpoint.
	mux.HandleFunc("GET /ws", hub.ServeWS)

	// Frontend — serve embedded HTML files.
	mux.Handle("GET /", serveFrontend())
	mux.Handle("GET /login", serveFrontend())
	mux.Handle("GET /2fa", serveFrontend())
	mux.Handle("GET /setup", serveFrontend())

	// Recovery + logging middleware.
	handler := loggingMiddleware(recoveryMiddleware(mux))

	// ── 15. Start HTTP server ────────────────────────────────────────────────
	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Graceful shutdown on SIGINT/SIGTERM.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		log.Printf("Received %s — shutting down…", sig)
		cancel() // Cancel all worker contexts.

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()
		pool.StopAll()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("HTTP shutdown: %v", err)
		}
	}()

	log.Printf("ClawDaemon listening on http://0.0.0.0:%s", cfg.Port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("ListenAndServe: %v", err)
	}
	log.Printf("ClawDaemon stopped.")
}

// serveFrontend returns a handler that serves the embedded web files.
func serveFrontend() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch path {
		case "/", "":
			serveFile(w, "index.html")
		case "/login":
			serveFile(w, "login.html")
		case "/2fa":
			serveFile(w, "2fa.html")
		case "/setup":
			serveFile(w, "wizard.html")
		default:
			http.NotFound(w, r)
		}
	})
}

func serveFile(w http.ResponseWriter, name string) {
	content, err := web.Files.ReadFile(name)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(content)
}

// loggingMiddleware logs each request.
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

// recoveryMiddleware recovers from panics and returns 500.
func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rv := recover(); rv != nil {
				log.Printf("panic: %v", rv)
				http.Error(w, `{"success":false,"error":"internal server error"}`, http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// telegramSender wraps *telegram.Bot to implement notify.Sender.
// Returns nil if bot is nil (Telegram disabled).
func telegramSender(bot *telegram.Bot) notify.Sender {
	if bot == nil {
		return nil
	}
	return bot
}

