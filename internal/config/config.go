package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Travelpayouts APIConfig
	Telegram      TelegramConfig
	Database      DatabaseConfig
	Server        ServerConfig
	Worker        WorkerConfig
}

type APIConfig struct {
	APIToken string
	Marker   int
	TRS      int
}

type TelegramConfig struct {
	Token string
}

type DatabaseConfig struct {
	URL string
}

type ServerConfig struct {
	Port int
}

type WorkerConfig struct {
	CheckInterval time.Duration
}

func Load() (*Config, error) {
	// Try to load .env file
	_ = godotenv.Load()

	apiToken := os.Getenv("TRAVELPAYOUTS_API_TOKEN")
	marker, _ := strconv.Atoi(os.Getenv("TRAVELPAYOUTS_MARKER"))
	trs, _ := strconv.Atoi(os.Getenv("TRAVELPAYOUTS_TRS"))

	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")

	dbURL := os.Getenv("DATABASE_URL")

	port, _ := strconv.Atoi(os.Getenv("HTTP_PORT"))
	if port == 0 {
		port = 8080
	}

	checkInterval, _ := strconv.Atoi(os.Getenv("CHECK_INTERVAL_MINUTES"))
	if checkInterval == 0 {
		checkInterval = 60
	}

	if apiToken == "" || botToken == "" || dbURL == "" {
		return nil, fmt.Errorf("missing required env vars: TRAVELPAYOUTS_API_TOKEN, TELEGRAM_BOT_TOKEN, DATABASE_URL")
	}

	return &Config{
		Travelpayouts: APIConfig{
			APIToken: apiToken,
			Marker:   marker,
			TRS:      trs,
		},
		Telegram: TelegramConfig{
			Token: botToken,
		},
		Database: DatabaseConfig{
			URL: dbURL,
		},
		Server: ServerConfig{
			Port: port,
		},
		Worker: WorkerConfig{
			CheckInterval: time.Duration(checkInterval) * time.Minute,
		},
	}, nil
}
