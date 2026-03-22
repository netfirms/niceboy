package strategy

import (
	"niceboy/internal/exchange"
)

// SignalType represents the action recommended by a strategy
type SignalType int

const (
	Wait SignalType = iota
	Buy
	Sell
)

func (s SignalType) String() string {
	return [...]string{"WAIT", "BUY", "SELL"}[s]
}

// Signal represents a trading decision
type Signal struct {
	Type   SignalType
	Symbol string
	Price  float64
	Reason string
}

// Strategy is the interface that all trading algorithms must implement
type Strategy interface {
	GetName() string
	OnMarketData(data exchange.MarketData) Signal
}
