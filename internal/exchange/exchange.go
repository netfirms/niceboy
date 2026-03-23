package exchange

import (
	"context"
)

// MarketData represents basic market information
type MarketData struct {
	Symbol string
	Price  float64
	Volume float64
	Time   int64 // Unix timestamp in milliseconds
}

// DepthEntry represents a single price level in the order book
type DepthEntry struct {
	Price    float64
	Quantity float64
}

// OrderBook represents the current market depth
type OrderBook struct {
	Symbol string
	Bids   []DepthEntry
	Asks   []DepthEntry
}

// OrderSide represents BUY or SELL
type OrderSide string

const (
	Buy  OrderSide = "BUY"
	Sell OrderSide = "SELL"
)

// Order represents an active trade on the books
type Order struct {
	ID       string
	Symbol   string
	Side     OrderSide
	Price    float64
	Quantity float64
}

// OrderType represents the type of order
type OrderType string

const (
	Market OrderType = "MARKET"
	Limit  OrderType = "LIMIT"
)

// SymbolInfo represents exchange-specific constraints for a trading pair
type SymbolInfo struct {
	Symbol         string
	BaseAsset      string
	QuoteAsset     string
	BasePrecision  int     // Decimal places for quantity
	QuotePrecision int     // Decimal places for price
	MinQty         float64 // Minimum quantity per order
	MinNotional    float64 // Minimum total order value (Qty * Price)
	StepSize       float64 // Quantity increment (e.g., 0.0001)
	TickSize       float64 // Price increment (e.g., 0.01)
}

// Exchange defines the interface for interacting with a cryptocurrency exchange
type Exchange interface {
	GetName() string
	GetPrice(ctx context.Context, symbol string) (float64, error)
	// SubscribePrice opens a websocket for real-time price updates
	SubscribePrice(ctx context.Context, symbol string, ch chan<- MarketData) error
	// ExecuteOrder places a trade on the exchange
	ExecuteOrder(ctx context.Context, symbol string, side OrderSide, orderType OrderType, quantity float64, price float64) error
	// GetBalances retrieves current asset quantities
	GetBalances(ctx context.Context) (map[string]float64, error)
	// GetOpenOrders retrieves unfilled orders for a specific symbol
	GetOpenOrders(ctx context.Context, symbol string) ([]Order, error)
	// GetSymbolInfo retrieves metadata like precision and minimums
	GetSymbolInfo(ctx context.Context, symbol string) (SymbolInfo, error)
	// GetOrderBook retrieves the top bids and asks
	GetOrderBook(ctx context.Context, symbol string, limit int) (OrderBook, error)
}
