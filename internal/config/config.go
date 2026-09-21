package config

import (
	"errors"
	"os"
	"strconv"
)

type Config struct {
	Port        string
	JWTSecret   string
	DatabaseURL string
	DemoMode    bool
}

func Load() (Config, error) {
	cfg := Config{
		Port:        getenv("PORT", "8080"),
		JWTSecret:   os.Getenv("JWT_SECRET"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		DemoMode:    getenvBool("DEMO_MODE", false),
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
