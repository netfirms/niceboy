package exchange

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type BitkubExchange struct {
	BaseURL string
	client  *http.Client
}

func NewBitkubExchange(apiKey, secretKey string) *BitkubExchange {
	return &BitkubExchange{
		BaseURL: "https://api.bitkub.com",
		client:  &http.Client{},
	}
}

func (b *BitkubExchange) GetName() string {
	return "bitkub"
}

func (b *BitkubExchange) GetPrice(ctx context.Context, symbol string) (float64, error) {
	url := fmt.Sprintf("%s/api/market/ticker?sym=%s", b.BaseURL, symbol)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, err
	}

	resp, err := b.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var data struct {
		Result map[string]struct {
			Last float64 `json:"last"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return 0, err
	}

	ticker, ok := data.Result[symbol]
	if !ok {
		return 0, fmt.Errorf("no ticker data for symbol: %s", symbol)
	}

	return ticker.Last, nil
}

func (b *BitkubExchange) SubscribePrice(ctx context.Context, symbol string, ch chan<- MarketData) error {
	return fmt.Errorf("websocket subscription for Bitkub not yet implemented")
}
