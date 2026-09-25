package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"steadwell/internal/adminauth"
	"steadwell/internal/adminorg"
	"steadwell/internal/channel"
	"steadwell/internal/config"
	"steadwell/internal/dbmigrate"
	"steadwell/internal/flows"
	"steadwell/internal/flowserver"
	"steadwell/internal/llm"
	"steadwell/internal/store"
	"steadwell/internal/telegramapi"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil)).With("service", "steadwell")
	slog.SetDefault(logger)
	_ = godotenv.Load()

	cfg := config.Load()
	if cfg.DatabaseURL == "" {
		logger.Error("DATABASE_URL is required (copy apps/backend/.env.example to .env)")
		os.Exit(1)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("postgres", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		logger.Error("postgres ping", "error", err)
		os.Exit(1)
	}
	if err := dbmigrate.Apply(ctx, pool); err != nil {
		logger.Error("migrate", "error", err)
		os.Exit(1)
	}
	cat, err := store.NewPostgres(ctx, pool)
	if err != nil {
		logger.Error("catalog", "error", err)
		os.Exit(1)
	}
	logger.Info("ready", "org", channel.DefaultOrgSlug, "port", cfg.HTTPPort)

	gen, genName, err := llm.SelectGenerator(ctx, llm.LiteLLMEnv{
		BaseURL: cfg.LiteLLMBaseURL,
		APIKey:  cfg.LiteLLMAPIKey,
		Model:   cfg.LiteLLMModel,
	})
	if err != nil {
		logger.Error("llm generator", "error", err)
		os.Exit(1)
	}
	logger.Info("llm generator", "backend", genName, "model", cfg.LiteLLMModel)

	deps := &flows.Deps{Catalog: cat, Chat: cat, Generator: gen}
	if cfg.TelegramBotToken != "" {
		deps.Telegram = telegramapi.NewBot(cfg.TelegramBotToken)
		logger.Info("telegram outbound enabled", "bot", cfg.TelegramBotUsername)
	} else {
		logger.Warn("TELEGRAM_BOT_TOKEN unset; webhook will accept updates but not reply in chat")
	}

	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{cfg.AdminOrigin, "http://localhost:5173"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Content-Type"},
		AllowCredentials: true,
	}))
	flowserver.RegisterRoutes(r, deps, cfg.TelegramWebhookSecret)
	adminauth.Register(r, pool)
	adminorg.Register(r, pool)

	srv := &http.Server{
		Addr:              ":" + cfg.HTTPPort,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}
	logger.Info("listening", "addr", srv.Addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("http", "error", err)
		os.Exit(1)
	}
}
