package exchange

import (
	"context"
	"fmt"

	"github.com/dvgamerr-app/go-bitkub/bitkub"
	"github.com/dvgamerr-app/go-bitkub/market"
)

type BitkubExchange struct {
}

func NewBitkubExchange(apiKey, secretKey string) *BitkubExchange {
	bitkub.Initlizer(apiKey, secretKey)
	return &BitkubExchange{}
}

func (b *BitkubExchange) GetName() string {
	return "bitkub"
}

func (b *BitkubExchange) GetPrice(ctx context.Context, symbol string) (float64, error) {
	ticker, err := market.GetTicker(symbol)
	if err != nil {
		return 0, err
	}

	if len(ticker) == 0 {
		return 0, fmt.Errorf("no ticker data for symbol: %s", symbol)
	}

	return ticker[0].Last, nil
}

func (b *BitkubExchange) SubscribePrice(ctx context.Context, symbol string, ch chan<- MarketData) error {
	return fmt.Errorf("websocket subscription for Bitkub not yet implemented")
}
