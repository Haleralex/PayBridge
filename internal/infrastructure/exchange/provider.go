package exchange

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"sync"
	"time"
)

// Provider fetches exchange rates from exchangerate-api.com with in-memory caching.
type Provider struct {
	apiKey    string
	apiURL    string
	cacheTTL  time.Duration
	client    *http.Client
	mu        sync.RWMutex
	cache     map[string]*cacheEntry
}

type cacheEntry struct {
	rates     map[string]*big.Rat
	expiresAt time.Time
}

// apiResponse represents the exchangerate-api.com response.
type apiResponse struct {
	Result          string             `json:"result"`
	BaseCode        string             `json:"base_code"`
	ConversionRates map[string]float64 `json:"conversion_rates"`
}

// NewProvider creates a new exchange rate provider.
func NewProvider(apiKey, apiURL string, cacheTTL time.Duration) *Provider {
	return &Provider{
		apiKey:   apiKey,
		apiURL:   apiURL,
		cacheTTL: cacheTTL,
		client:   &http.Client{Timeout: 10 * time.Second},
		cache:    make(map[string]*cacheEntry),
	}
}

// GetRate returns the exchange rate from one currency to another.
func (p *Provider) GetRate(ctx context.Context, from, to string) (*big.Rat, error) {
	if from == to {
		return new(big.Rat).SetInt64(1), nil
	}

	// Check cache first
	if rate := p.getCached(from, to); rate != nil {
		return rate, nil
	}

	// Fetch from API
	rates, err := p.fetchRates(ctx, from)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch exchange rates: %w", err)
	}

	rate, ok := rates[to]
	if !ok {
		return nil, fmt.Errorf("exchange rate not available for %s → %s", from, to)
	}

	return rate, nil
}

func (p *Provider) getCached(from, to string) *big.Rat {
	p.mu.RLock()
	defer p.mu.RUnlock()

	entry, ok := p.cache[from]
	if !ok || time.Now().After(entry.expiresAt) {
		return nil
	}

	rate, ok := entry.rates[to]
	if !ok {
		return nil
	}

	// Return a copy to avoid race conditions
	return new(big.Rat).Set(rate)
}

func (p *Provider) fetchRates(ctx context.Context, baseCurrency string) (map[string]*big.Rat, error) {
	url := fmt.Sprintf("%s/%s/latest/%s", p.apiURL, p.apiKey, baseCurrency)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var apiResp apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if apiResp.Result != "success" {
		return nil, fmt.Errorf("API returned error result: %s", apiResp.Result)
	}

	// Convert float64 rates to big.Rat for precision
	rates := make(map[string]*big.Rat, len(apiResp.ConversionRates))
	for currency, rate := range apiResp.ConversionRates {
		r := new(big.Rat)
		r.SetFloat64(rate)
		rates[currency] = r
	}

	// Update cache
	p.mu.Lock()
	p.cache[baseCurrency] = &cacheEntry{
		rates:     rates,
		expiresAt: time.Now().Add(p.cacheTTL),
	}
	p.mu.Unlock()

	return rates, nil
}
