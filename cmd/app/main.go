package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"strconv"
	"github.com/travel-to-tour/tour-tracker/internal/affiliate"
	"github.com/travel-to-tour/tour-tracker/internal/api"
	"github.com/travel-to-tour/tour-tracker/internal/config"
	sec "github.com/travel-to-tour/tour-tracker/internal/middleware"
	deliveryHTTP "github.com/travel-to-tour/tour-tracker/internal/delivery/http"
	telegramDelivery "github.com/travel-to-tour/tour-tracker/internal/delivery/telegram"
	"github.com/travel-to-tour/tour-tracker/internal/provider"
	"github.com/travel-to-tour/tour-tracker/internal/storage"
	"github.com/travel-to-tour/tour-tracker/internal/worker"
)

func main() {
	log.Println("=== Tour Tracker Starting ===")

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Config error: %v", err)
	}

	s, err := storage.NewFromURL(cfg.Database.URL)
	if err != nil {
		log.Fatalf("Database error: %v", err)
	}
	log.Println("Database connected")

	var affClient *affiliate.Client
	if cfg.Travelpayouts.APIToken != "" {
		affClient = affiliate.NewClient(cfg.Travelpayouts.APIToken, cfg.Travelpayouts.TRS, cfg.Travelpayouts.Marker)
		log.Println("Travelpayouts client initialized")
	} else {
		log.Println("Travelpayouts: not configured")
	}

	providers := []provider.TourProvider{provider.NewStubProvider("stub")}
	log.Println("Tour providers initialized (stub mode)")

	var tgBot *telegramDelivery.Bot
	if cfg.Telegram.Token != "" {
		tgBot, err = telegramDelivery.NewBot(cfg.Telegram.Token, s)
		if err != nil {
			log.Printf("WARNING: Telegram bot failed: %v", err)
		} else {
			log.Println("Telegram bot initialized")
		}
	} else {
		log.Println("Telegram: not configured")
	}

	var monitor *worker.Monitor
	if affClient != nil {
		monitor = worker.NewMonitor(s, providers, affClient, tgBot)
		log.Println("Monitor initialized")
	} else {
		log.Println("Monitor: disabled (no affiliate client)")
	}

	securityCfg := sec.DefaultConfig()
	jwtCfg := sec.DefaultJWTConfig()

	apiRouter := api.NewAPI(s, securityCfg, jwtCfg).Router()
	webRouter := deliveryHTTP.NewHTTP(s, securityCfg).Router()

	mux := chi.NewRouter()
	mux.Use(middleware.Logger)
	mux.Use(middleware.Recoverer)
	mux.Mount("/api", apiRouter)
	mux.Mount("/", webRouter)

	httpServer := &http.Server{
		Addr:         ":" + strconv.Itoa(cfg.Server.Port),
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	if tgBot != nil {
		go func() { tgBot.Start() }()
		log.Println("Telegram bot started")
	}

	var monitorCtx context.Context
	var monitorCancel context.CancelFunc
	if monitor != nil {
		monitorCtx, monitorCancel = context.WithCancel(context.Background())
		go func() { monitor.Run(monitorCtx, cfg.Worker.CheckInterval) }()
		log.Printf("Monitor started (interval: %v)", cfg.Worker.CheckInterval)
	}

	go func() {
		log.Printf("HTTP server starting on port %d", cfg.Server.Port)
		log.Println("Site: http://localhost:" + strconv.Itoa(cfg.Server.Port))
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	log.Println("=== Tour Tracker Ready ===")
	log.Println("Site: http://localhost:" + string(cfg.Server.Port))
	if tgBot != nil { log.Println("Telegram bot: active") }
	if affClient != nil { log.Println("Affiliate links: enabled") }

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down...")
	if monitorCancel != nil { monitorCancel() }
	if monitor != nil { monitor.Stop() }

	httpCtx, httpCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer httpCancel()
	if err := httpServer.Shutdown(httpCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}
	if tgBot != nil { tgBot.Stop() }
	log.Println("Shutdown complete")
}