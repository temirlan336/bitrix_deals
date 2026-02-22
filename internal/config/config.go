package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	BitrixWebhookBaseURL string
	DatabaseURL          string
}

func Load() (Config, error) {
	base := strings.TrimSpace(os.Getenv("BITRIX_WEBHOOK_BASE_URL"))
	if base == "" {
		return Config{}, fmt.Errorf("BITRIX_WEBHOOK_BASE_URL is empty")
	}
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}

	dbURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if dbURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is empty")
	}

	return Config{
		BitrixWebhookBaseURL: base,
		DatabaseURL:          dbURL,
	}, nil
}
