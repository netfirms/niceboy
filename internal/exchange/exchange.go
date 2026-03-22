package exchange

import (
	"context"
)

// MarketData represents basic market information
type MarketData struct {
	Symbol string
	Price  float64
}

// Exchange defines the interface for interacting with a cryptocurrency exchange
type Exchange interface {
	GetName() string
	GetPrice(ctx context.Context, symbol string) (float64, error)
	// SubscribePrice opens a websocket for real-time price updates
	SubscribePrice(ctx context.Context, symbol string, ch chan<- MarketData) error
}
