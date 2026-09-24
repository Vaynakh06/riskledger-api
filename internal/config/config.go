package config

import (
	"errors"
	"os"
	"strconv"
)

type Config struct {
	Port             string
	JWTSecret        string
	DatabaseURL      string
	DemoMode         bool
	MarketDataURL    string
	MarketDataAPIKey string
	SMTPHost         string
	SMTPPort         string
	SMTPUser         string
	SMTPPassword     string
	AlertFrom        string
	TelegramBotToken string
	TelegramChatID   string
	SlackWebhookURL  string
	AlertWebhookURL  string
	BrokerAPIURL     string
	BrokerAPIToken   string
}

func Load() (Config, error) {
	cfg := Config{
		Port:             getenv("PORT", "8080"),
		JWTSecret:        os.Getenv("JWT_SECRET"),
		DatabaseURL:      os.Getenv("DATABASE_URL"),
		DemoMode:         getenvBool("DEMO_MODE", false),
		MarketDataURL:    getenv("MARKET_DATA_URL", "https://api.coingecko.com/api/v3/simple/price"),
		MarketDataAPIKey: os.Getenv("MARKET_DATA_API_KEY"),
		SMTPHost:         os.Getenv("SMTP_HOST"),
		SMTPPort:         getenv("SMTP_PORT", "587"),
		SMTPUser:         os.Getenv("SMTP_USER"),
		SMTPPassword:     os.Getenv("SMTP_PASSWORD"),
		AlertFrom:        os.Getenv("ALERT_FROM"),
		TelegramBotToken: os.Getenv("TELEGRAM_BOT_TOKEN"),
		TelegramChatID:   os.Getenv("TELEGRAM_CHAT_ID"),
		SlackWebhookURL:  os.Getenv("SLACK_WEBHOOK_URL"),
		AlertWebhookURL:  os.Getenv("ALERT_WEBHOOK_URL"),
		BrokerAPIURL:     os.Getenv("BROKER_API_URL"),
		BrokerAPIToken:   os.Getenv("BROKER_API_TOKEN"),
	}
	if !cfg.DemoMode && len(cfg.JWTSecret) < 32 {
		return Config{}, errors.New("JWT_SECRET must be configured with at least 32 characters")
	}
	if cfg.DemoMode && cfg.JWTSecret == "" {
		cfg.JWTSecret = "local-demo-secret-change-me-please-32"
	}
	if !cfg.DemoMode && cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL must be configured outside DEMO_MODE")
	}
	return cfg, nil
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getenvBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
