package exchange

import (
	"context"
	"fmt"
	"strconv"

	"github.com/adshao/go-binance/v2"
)

type BinanceExchange struct {
	client *binance.Client
}

func NewBinanceExchange(apiKey, secretKey string) *BinanceExchange {
	return &BinanceExchange{
		client: binance.NewClient(apiKey, secretKey),
	}
}

func (b *BinanceExchange) GetName() string {
	return "binance"
}

func (b *BinanceExchange) GetPrice(ctx context.Context, symbol string) (float64, error) {
	prices, err := b.client.NewListPricesService().Symbol(symbol).Do(ctx)
	if err != nil {
		return 0, err
	}
	if len(prices) == 0 {
		return 0, fmt.Errorf("no price found for symbol: %s", symbol)
	}
	return strconv.ParseFloat(prices[0].Price, 64)
}

func (b *BinanceExchange) SubscribePrice(ctx context.Context, symbol string, ch chan<- MarketData) error {
	// Implementation for WebSocket would go here
	// For now, this is a placeholder to show the interface capability
	return fmt.Errorf("websocket subscription not yet implemented")
}
