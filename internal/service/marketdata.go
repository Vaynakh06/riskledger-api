package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type PriceProvider interface {
	CurrentPrice(symbol string) (float64, bool)
}

type HTTPPriceProvider struct {
	baseURL string
	apiKey  string
	client  *http.Client
	mu      sync.RWMutex
	cache   map[string]cachedPrice
}

type cachedPrice struct {
	value     float64
	expiresAt time.Time
}

func NewHTTPPriceProvider(baseURL, apiKey string) *HTTPPriceProvider {
	return &HTTPPriceProvider{baseURL: strings.TrimRight(baseURL, "?&"), apiKey: apiKey, client: &http.Client{Timeout: 3 * time.Second}, cache: make(map[string]cachedPrice)}
}

func (p *HTTPPriceProvider) CurrentPrice(symbol string) (float64, bool) {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if symbol == "" {
		return 0, false
	}
	p.mu.RLock()
	value, ok := p.cache[symbol]
	p.mu.RUnlock()
	if ok && time.Now().Before(value.expiresAt) {
		return value.value, true
	}
	price, err := p.fetch(symbol)
	if err != nil || price <= 0 {
		if ok && value.value > 0 {
			return value.value, true
		}
		return 0, false
	}
	p.mu.Lock()
	p.cache[symbol] = cachedPrice{value: price, expiresAt: time.Now().Add(60 * time.Second)}
	p.mu.Unlock()
	return price, true
}

func (p *HTTPPriceProvider) fetch(symbol string) (float64, error) {
	coinID := map[string]string{"BTC": "bitcoin", "ETH": "ethereum", "SOL": "solana", "USDT": "tether", "USDC": "usd-coin"}[symbol]
	if coinID == "" {
		return p.fetchYahoo(symbol)
	}
	endpoint, err := url.Parse(p.baseURL)
	if err != nil {
		return 0, err
	}
	query := endpoint.Query()
	query.Set("ids", coinID)
	query.Set("vs_currencies", "usd")
	endpoint.RawQuery = query.Encode()
	req, err := http.NewRequest(http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return 0, err
	}
	if p.apiKey != "" {
		req.Header.Set("x-cg-demo-api-key", p.apiKey)
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}
	response, err := p.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("market data status %d", response.StatusCode)
	}
	var payload map[string]map[string]float64
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return 0, err
	}
	return payload[coinID]["usd"], nil
}

func (p *HTTPPriceProvider) fetchYahoo(symbol string) (float64, error) {
	endpoint := "https://query1.finance.yahoo.com/v8/finance/chart/" + url.PathEscape(symbol) + "?range=1d&interval=1m"
	request, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return 0, err
	}
	response, err := p.client.Do(request)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("market data status %d", response.StatusCode)
	}
	var payload struct {
		Chart struct {
			Result []struct {
				Meta struct {
					RegularMarketPrice float64 `json:"regularMarketPrice"`
				} `json:"meta"`
			} `json:"result"`
		} `json:"chart"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return 0, err
	}
	if len(payload.Chart.Result) == 0 || payload.Chart.Result[0].Meta.RegularMarketPrice <= 0 {
		return 0, fmt.Errorf("no quote for %s", symbol)
	}
	return payload.Chart.Result[0].Meta.RegularMarketPrice, nil
}
