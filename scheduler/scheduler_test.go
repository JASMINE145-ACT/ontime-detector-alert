package scheduler

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"ontime-detector-alert/alerts"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestNotifyOpenClaw_NoUserID_DoesNotCallHTTP(t *testing.T) {
	origClient := http.DefaultClient
	defer func() { http.DefaultClient = origClient }()

	http.DefaultClient = &http.Client{
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			t.Fatalf("HTTP should not be called when userID is empty")
			return nil, nil
		}),
	}

	notifyOpenClaw("", "AAPL", 123.45)
}

func TestNotifyOpenClaw_SendsExpectedRequest(t *testing.T) {
	origClient := http.DefaultClient
	defer func() { http.DefaultClient = origClient }()

	var capturedReq *http.Request
	var capturedBody []byte

	http.DefaultClient = &http.Client{
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			capturedReq = r
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("failed to read request body: %v", err)
			}
			capturedBody = body

			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBufferString("OK")),
				Header:     make(http.Header),
				Request:    r,
			}, nil
		}),
	}

	notifyOpenClaw("user-123", "AAPL", 123.45)

	if capturedReq == nil {
		t.Fatalf("expected HTTP request to be sent")
	}
	if capturedReq.Method != http.MethodPost {
		t.Errorf("expected POST method, got %s", capturedReq.Method)
	}
	if got := capturedReq.URL.String(); got != "https://ontime-detector-alert.onrender.com/agent/notify" {
		t.Errorf("unexpected URL: %s", got)
	}
	if ct := capturedReq.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}

	var payload map[string]any
	if err := json.Unmarshal(capturedBody, &payload); err != nil {
		t.Fatalf("failed to unmarshal body: %v", err)
	}

	if got := payload["user_id"]; got != "user-123" {
		t.Errorf("unexpected user_id: %v", got)
	}
	if msg, ok := payload["message"].(string); !ok || msg == "" {
		t.Fatalf("expected non-empty message string, got %T %v", payload["message"], payload["message"])
	}
}

type fakeRepo struct {
	alerts            []alerts.Alert
	updatedID         string
	updatedTriggered  *time.Time
	updatedLastNotify *time.Time
}

func (r *fakeRepo) Create(a *alerts.Alert) error { return nil }
func (r *fakeRepo) Delete(id string) error       { return nil }
func (r *fakeRepo) ListByUser(userID string) ([]alerts.Alert, error) {
	return nil, nil
}
func (r *fakeRepo) ListActive() ([]alerts.Alert, error) {
	return append([]alerts.Alert(nil), r.alerts...), nil
}
func (r *fakeRepo) UpdateNotificationState(id string, triggeredAt, lastNotifiedAt *time.Time) error {
	r.updatedID = id
	r.updatedTriggered = triggeredAt
	r.updatedLastNotify = lastNotifiedAt
	return nil
}
func (r *fakeRepo) Close() error { return nil }

type fakeProvider struct {
	prices map[string]float64
}

func (p *fakeProvider) GetPrices(symbols []string) (map[string]float64, error) {
	return p.prices, nil
}

type fakeNotifier struct {
	lastContent string
}

func (n *fakeNotifier) SendText(content string) error {
	n.lastContent = content
	return nil
}

func TestSchedulerTick_TriggersAlertAndUpdatesRepo(t *testing.T) {
	now := time.Now().UTC()

	repo := &fakeRepo{
		alerts: []alerts.Alert{
			{
				ID:        "alert-1",
				Symbol:    "AAPL",
				Direction: alerts.DirectionAbove,
				Threshold: 100,
				UserID:    "",
				Active:    true,
				// ensure cooldown does not block
				CooldownSeconds: 0,
				LastNotifiedAt:  nil,
				CreatedAt:       now,
				UpdatedAt:       now,
			},
		},
	}
	provider := &fakeProvider{
		prices: map[string]float64{
			"AAPL": 150,
		},
	}
	notif := &fakeNotifier{}

	s := NewScheduler(repo, provider, notif, time.Second)

	if err := s.tick(); err != nil {
		t.Fatalf("tick returned error: %v", err)
	}

	if repo.updatedID != "alert-1" {
		t.Errorf("expected repo to update alert-1, got %q", repo.updatedID)
	}
	if repo.updatedTriggered == nil || repo.updatedLastNotify == nil {
		t.Errorf("expected triggered and lastNotified timestamps to be set")
	}
	if notif.lastContent == "" {
		t.Errorf("expected notifier to be called")
	}
}

