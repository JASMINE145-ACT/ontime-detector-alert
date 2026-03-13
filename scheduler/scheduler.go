package scheduler

import (
	"fmt"
	"log"
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
	for _, a := range triggered {
		price := prices[a.Symbol]
		content := fmt.Sprintf("Symbol: %s\nCondition: %s %.4f\nPrice: %.4f\nTime: %s",
			a.Symbol, directionText(a.Direction), a.Threshold, price, now.Format(time.RFC3339))
		if err := s.notifier.SendText(content); err != nil {
			log.Printf("send notification failed for alert %s: %v", a.ID, err)
			continue
		}
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

