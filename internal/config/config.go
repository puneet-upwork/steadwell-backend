package config

import "os"

type Config struct {
	HTTPPort              string
	TelegramBotToken      string
	TelegramBotUsername   string
	TelegramWebhookSecret string
	WhatsAppNumber        string
	LineLiffURL           string
	DatabaseURL           string
	AdminOrigin           string
	LiteLLMBaseURL        string
	LiteLLMAPIKey         string
	LiteLLMModel          string
}

func Load() Config {
	port := os.Getenv("HTTP_PORT")
	if port == "" {
		port = "8080"
	}
	origin := os.Getenv("ADMIN_ORIGIN")
	if origin == "" {
		origin = "http://127.0.0.1:5173"
	}
	return Config{
		HTTPPort:              port,
		TelegramBotToken:      os.Getenv("TELEGRAM_BOT_TOKEN"),
		TelegramBotUsername:   os.Getenv("TELEGRAM_BOT_USERNAME"),
		TelegramWebhookSecret: os.Getenv("TELEGRAM_WEBHOOK_SECRET"),
		WhatsAppNumber:        os.Getenv("WHATSAPP_BUSINESS_NUMBER"),
		LineLiffURL:           os.Getenv("LINE_LIFF_URL"),
		DatabaseURL:           os.Getenv("DATABASE_URL"),
		AdminOrigin:           origin,
		LiteLLMBaseURL:        os.Getenv("LITELLM_BASE_URL"),
		LiteLLMAPIKey:         os.Getenv("LITELLM_API_KEY"),
		LiteLLMModel:          os.Getenv("LITELLM_MODEL"),
	}
}
