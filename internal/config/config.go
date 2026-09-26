package config

import (
	"os"
)

type Config struct {
	DatabaseURL      string
	TelegramBotToken string
	TelegramChatID   string
	Timezone         string
	Port             string
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func Load() Config {
	return Config{
		DatabaseURL:      getenv("DATABASE_URL", "postgres://spikeidx:spikeidx@localhost:5432/spikeidx?sslmode=disable"),
		TelegramBotToken: os.Getenv("TELEGRAM_BOT_TOKEN"),
		TelegramChatID:   os.Getenv("TELEGRAM_CHAT_ID"),
		Timezone:         getenv("TZ", "Asia/Jakarta"),
		Port:             getenv("PORT", "8080"),
	}
}
