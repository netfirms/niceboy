package exchange

import (
	"context"
)

// MarketData represents basic market information
type MarketData struct {
	Symbol string
	Price  float64
	Time   int64 // Unix timestamp in milliseconds
}

// OrderSide represents BUY or SELL
type OrderSide string

const (
	Buy  OrderSide = "BUY"
	Sell OrderSide = "SELL"
)

// OrderType represents the type of order
type OrderType string

const (
	Market OrderType = "MARKET"
	Limit  OrderType = "LIMIT"
)

// Exchange defines the interface for interacting with a cryptocurrency exchange
type Exchange interface {
	GetName() string
	GetPrice(ctx context.Context, symbol string) (float64, error)
	// SubscribePrice opens a websocket for real-time price updates
	SubscribePrice(ctx context.Context, symbol string, ch chan<- MarketData) error
	// ExecuteOrder places a trade on the exchange
	ExecuteOrder(ctx context.Context, symbol string, side OrderSide, orderType OrderType, quantity float64, price float64) error
}
