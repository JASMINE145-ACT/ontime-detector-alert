package priceprovider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type YahooProvider struct {
	baseURL string
	client  *http.Client
}

func NewYahooProvider(baseURL string) Provider {
	if baseURL == "" {
		baseURL = "https://query1.finance.yahoo.com"
	}
	return &YahooProvider{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (p *YahooProvider) GetPrices(symbols []string) (map[string]float64, error) {
	result := make(map[string]float64)
	if len(symbols) == 0 {
		return result, nil
	}

	symbols = dedupe(symbols)
	base := strings.TrimRight(p.baseURL, "/")

	for _, symbol := range symbols {
		rawURL := fmt.Sprintf("%s/v8/finance/chart/%s?interval=1m", base, url.PathEscape(symbol))

		req, err := http.NewRequest(http.MethodGet, rawURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
		req.Header.Set("Accept", "application/json")

		resp, err := p.client.Do(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("yahoo: unexpected status %s", resp.Status)
		}

		var data struct {
			Chart struct {
				Result []struct {
					Meta struct {
						RegularMarketPrice float64 `json:"regularMarketPrice"`
					} `json:"meta"`
				} `json:"result"`
			} `json:"chart"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			resp.Body.Close()
			return nil, err
		}
		resp.Body.Close()

		if len(data.Chart.Result) > 0 {
			result[symbol] = data.Chart.Result[0].Meta.RegularMarketPrice
		}
	}
	return result, nil
}

func dedupe(in []string) []string {
	if len(in) == 0 {
		return in
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

