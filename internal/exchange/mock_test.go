package exchange

import (
	"context"
	"errors"
	"testing"
)

type MockExchange struct {
	Price    float64
	RetError error
}

func (m *MockExchange) GetName() string { return "mock" }
func (m *MockExchange) GetPrice(ctx context.Context, symbol string) (float64, error) {
	return m.Price, m.RetError
}

func (m *MockExchange) SubscribePrice(ctx context.Context, symbol string, ch chan<- MarketData) error {
	return m.RetError
}

func (m *MockExchange) ExecuteOrder(ctx context.Context, symbol string, side OrderSide, orderType OrderType, quantity float64, price float64) error {
	return m.RetError
}

func TestMockExchange_GetPrice(t *testing.T) {
	mock := &MockExchange{Price: 1234.56}
	price, err := mock.GetPrice(context.Background(), "BTCUSDT")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if price != 1234.56 {
		t.Errorf("expected 1234.56, got %f", price)
	}

	mock.RetError = errors.New("network error")
	_, err = mock.GetPrice(context.Background(), "BTCUSDT")
	if err == nil {
		t.Error("expected error, got nil")
	}
}
