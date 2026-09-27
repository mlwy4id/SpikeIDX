package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Settings struct {
	DatabaseURL      string
	TelegramBotToken string
	TelegramChatID   string
	Timezone         string
	Port             string
}

func getenv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

func Load() Settings {
	godotenv.Load(".env")
	
	return Settings{
		DatabaseURL:      os.Getenv("DATABASE_URL"),
		TelegramBotToken: os.Getenv("TELEGRAM_BOT_TOKEN"),
		TelegramChatID:   os.Getenv("TELEGRAM_CHAT_ID"),
		Timezone:         getenv("TZ", "Asia/Jakarta"),
		Port:             getenv("PORT", "8080"),
	}
}
