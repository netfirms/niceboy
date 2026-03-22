package exchange

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
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
	// Bitkub WebSocket API requires a specific connection setup.
	// For simplicity, we fallback to a rapid polling simulated stream for now.
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				price, err := b.GetPrice(ctx, symbol)
				if err == nil {
					ch <- MarketData{
						Symbol: symbol,
						Price:  price,
						Time:   time.Now().UnixNano() / 1e6,
					}
				}
			}
		}
	}()
	return nil
}

func (b *BitkubExchange) ExecuteOrder(ctx context.Context, symbol string, side OrderSide, orderType OrderType, quantity float64, price float64) error {
	// In a real implementation we would sign the payload and hit /api/market/place-bid or place-ask
	// Since this is a local bot and Bitkub SDK support is limited, we simulate the execution.
	if quantity <= 0 {
		return fmt.Errorf("invalid quantity: %f", quantity)
	}
	
	// Simulate network latency
	time.Sleep(100 * time.Millisecond)
	
	// Return nil to indicate a successful "dry run" execution for local testing
	return nil
}
