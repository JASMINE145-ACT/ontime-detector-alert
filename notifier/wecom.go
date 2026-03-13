package notifier

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

type Notifier interface {
	SendText(content string) error
}

type weComNotifier struct {
	webhookURL string
	client     *http.Client
}

func NewWeComNotifier(webhookURL string) Notifier {
	return &weComNotifier{
		webhookURL: webhookURL,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (n *weComNotifier) SendText(content string) error {
	if n.webhookURL == "" {
		log.Printf("WeCom webhook URL is empty, message skipped: %s", content)
		return nil
	}

	payload := map[string]any{
		"msgtype": "text",
		"text": map[string]string{
			"content": content,
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, n.webhookURL, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("WeCom webhook returned status %s", resp.Status)
	}
	return nil
}

