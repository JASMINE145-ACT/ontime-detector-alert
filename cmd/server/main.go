package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"ontime-detector-alert/alerts"
	"ontime-detector-alert/api"
	"ontime-detector-alert/notifier"
	"ontime-detector-alert/priceprovider"
	"ontime-detector-alert/scheduler"
)

func main() {
	cfg, err := LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	dbPath := cfg.DatabasePath
	if dbPath == "" {
		dbPath = "alerts.db"
	}

	alertRepo, err := alerts.NewSQLiteRepository(dbPath)
	if err != nil {
		log.Fatalf("init alert repository: %v", err)
	}
	defer alertRepo.Close()

	priceProv := priceprovider.NewYahooProvider(cfg.PriceProvider.BaseURL)

	wecomURL := cfg.WeComWebhookURL
	if env := os.Getenv("WECOM_WEBHOOK_URL"); env != "" {
		wecomURL = env
	}
	if wecomURL == "" {
		log.Println("warning: WeCom webhook URL is empty, notifications will be skipped")
	}
	ntf := notifier.NewWeComNotifier(wecomURL)

	interval := time.Duration(cfg.SchedulerIntervalSeconds) * time.Second
	if interval <= 0 {
		interval = 30 * time.Second
	}

	s := scheduler.NewScheduler(alertRepo, priceProv, ntf, interval)
	go s.Run()

	apiHandler := api.NewServer(alertRepo)

	addr := cfg.ListenAddress
	if addr == "" {
		addr = ":8080"
	}
	if port := os.Getenv("PORT"); port != "" {
		addr = ":" + port
	}

	log.Printf("starting alert backend on %s", addr)
	if err := http.ListenAndServe(addr, apiHandler); err != nil {
		log.Fatalf("http server error: %v", err)
	}
}

