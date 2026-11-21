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
	Max           MaxConfig
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

type MaxConfig struct {
	APIKey  string
	BaseURL string
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
	maxAPIKey := os.Getenv("MAX_API_KEY")
	maxBaseURL := os.Getenv("MAX_BASE_URL")

	// Try DATABASE_URL first, then construct from DB_* vars
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbName := os.Getenv("DB_NAME")
		dbUser := os.Getenv("DB_USER")
		dbPass := os.Getenv("DB_PASSWORD")
		if dbName != "" && dbUser != "" && dbPass != "" {
			dbURL = fmt.Sprintf("postgres://%s:%s@postgres:5432/%s?sslmode=disable", dbUser, dbPass, dbName)
		}
	}

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
		Max: MaxConfig{
			APIKey:  maxAPIKey,
			BaseURL: maxBaseURL,
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
