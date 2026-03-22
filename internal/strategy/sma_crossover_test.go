package strategy

import (
	"niceboy/internal/exchange"
	"testing"
)

func TestSMACrossover_OnMarketData(t *testing.T) {
	strat, err := New("sma_crossover", map[string]interface{}{
		"short_period": 5,
		"long_period":  10,
	})
	if err != nil {
		t.Fatalf("failed to init strategy: %v", err)
	}

	tests := []struct {
		name     string
		price    float64
		expected SignalType
	}{
		{"Initial Wait (Data collection)", 100, Wait},
		{"Wait for 2nd point", 101, Wait},
		{"Wait for 3rd point", 102, Wait},
		{"Wait for 4th point", 103, Wait},
		{"Wait for 5th point", 104, Wait},
		{"Wait for 6th point", 105, Wait},
		{"Wait for 7th point", 106, Wait},
		{"Wait for 8th point", 107, Wait},
		{"Wait for 9th point", 108, Wait},
		{"Cross Above (BUY)", 109, Buy},
		{"Still Above (BUY)", 115, Buy},
		{"Still Above (BUY)", 116, Buy},
		{"Price Drop (Still BUY)", 90, Buy},
		{"Cross Below (SELL)", 80, Sell},
		{"Still Below (SELL)", 70, Sell},
		{"Still Below (SELL)", 60, Sell},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			signal := strat.OnMarketData(exchange.MarketData{
				Symbol: "BTCUSDT",
				Price:  tt.price,
			})
			if signal.Type != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, signal.Type)
			}
		})
	}
}

func TestSignalType_String(t *testing.T) {
	if Buy.String() != "BUY" {
		t.Errorf("expected BUY, got %s", Buy.String())
	}
	if Sell.String() != "SELL" {
		t.Errorf("expected SELL, got %s", Sell.String())
	}
	if Wait.String() != "WAIT" {
		t.Errorf("expected WAIT, got %s", Wait.String())
	}
}

func TestStrategy_Registry(t *testing.T) {
	t.Run("Valid Strategy", func(t *testing.T) {
		strat, err := New("sma_crossover", map[string]interface{}{})
		if err != nil {
			t.Fatalf("failed to create strategy: %v", err)
		}
		if strat.GetName() != "sma_crossover" {
			t.Errorf("expected sma_crossover, got %s", strat.GetName())
		}
	})

	t.Run("Invalid Strategy", func(t *testing.T) {
		_, err := New("unknown", nil)
		if err == nil {
			t.Error("expected error for unknown strategy, got nil")
		}
	})
}
