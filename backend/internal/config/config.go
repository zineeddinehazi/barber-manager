package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL    string
	Port           string
	JWTSecret      string
	JWTExpiryHours int
}

func Load() (*Config, error) {
	_ = godotenv.Load() // optional: ignore error, real env vars are a valid source too

	cfg := &Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		Port:        os.Getenv("PORT"),
		JWTSecret:   os.Getenv("JWT_SECRET"),
	}
	if cfg.DatabaseURL == "" || cfg.JWTSecret == "" {
		return nil, fmt.Errorf("missing required env vars: DATABASE_URL and JWT_SECRET must be set")
	}
	if cfg.Port == "" {
		cfg.Port = "8080"
	}

	cfg.JWTExpiryHours = 24
	if v := os.Getenv("JWT_EXPIRY_HOURS"); v != "" {
		hours, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid JWT_EXPIRY_HOURS: %w", err)
		}
		cfg.JWTExpiryHours = hours
	}

	return cfg, nil
}
