package scheduler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"ontime-detector-alert/alerts"
	"ontime-detector-alert/engine"
	"ontime-detector-alert/notifier"
	"ontime-detector-alert/priceprovider"
)

type Scheduler struct {
	repo     alerts.Repository
	provider priceprovider.Provider
	notifier notifier.Notifier
	interval time.Duration
	stopCh   chan struct{}
}

func NewScheduler(repo alerts.Repository, provider priceprovider.Provider, notifier notifier.Notifier, interval time.Duration) *Scheduler {
	return &Scheduler{
		repo:     repo,
		provider: provider,
		notifier: notifier,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

func (s *Scheduler) Run() {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := s.tick(); err != nil {
				log.Printf("scheduler tick error: %v", err)
			}
		case <-s.stopCh:
			return
		}
	}
}

func (s *Scheduler) Stop() {
	close(s.stopCh)
}

func (s *Scheduler) tick() error {
	alertsList, err := s.repo.ListActive()
	if err != nil {
		return fmt.Errorf("list alerts: %w", err)
	}
	if len(alertsList) == 0 {
		log.Printf("scheduler tick: 0 alerts, nothing to check")
		return nil
	}

	symbolSet := make(map[string]struct{})
	for _, a := range alertsList {
		if a.Symbol != "" {
			symbolSet[a.Symbol] = struct{}{}
		}
	}
	var symbols []string
	for sym := range symbolSet {
		symbols = append(symbols, sym)
	}

	prices, err := s.provider.GetPrices(symbols)
	if err != nil {
		return fmt.Errorf("get prices: %w", err)
	}
	now := time.Now().UTC()

	triggered := engine.EvaluateAlerts(alertsList, prices, now)
	log.Printf("scheduler tick: %d alerts, %d triggered, prices: %v", len(alertsList), len(triggered), prices)
	for _, a := range triggered {
		price := prices[a.Symbol]
		content := fmt.Sprintf("Symbol: %s\nCondition: %s %.4f\nPrice: %.4f\nTime: %s",
			a.Symbol, directionText(a.Direction), a.Threshold, price, now.Format(time.RFC3339))
		if err := s.notifier.SendText(content); err != nil {
			log.Printf("send notification failed for alert %s: %v", a.ID, err)
			continue
		}
		notifyOpenClaw(a.UserID, a.Symbol, price)
		if err := s.repo.UpdateNotificationState(a.ID, &now, &now); err != nil {
			log.Printf("update notification state failed for alert %s: %v", a.ID, err)
		}
	}
	return nil
}

func directionText(d alerts.Direction) string {
	switch d {
	case alerts.DirectionAbove:
		return ">"
	case alerts.DirectionBelow:
		return "<"
	default:
		return "?"
	}
}

func notifyOpenClaw(userID, symbol string, price float64) {
	if userID == "" {
		return
	}

	payload := map[string]interface{}{
		"user_id": userID,
		"message": fmt.Sprintf("⚠️ Alert Triggered\nSymbol: %s\nPrice: %.4f", symbol, price),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("notifyOpenClaw: marshal payload failed: %v", err)
		return
	}

	resp, err := http.Post(
		"https://ontime-detector-alert.onrender.com/agent/notify",
		"application/json",
		bytes.NewBuffer(body),
	)
	if err != nil {
		log.Printf("notifyOpenClaw: http post failed: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("notifyOpenClaw: non-2xx status: %s", resp.Status)
	}
}

