package strategy

import (
	"niceboy/internal/exchange"
	"testing"
)

func TestSMACrossover_OnMarketData(t *testing.T) {
	strat, _ := New("sma_crossover", map[string]interface{}{
		"short_period": 5,
		"long_period":  10,
	})

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
		{"Still Above (WAIT)", 115, Wait},
		{"Still Above (WAIT)", 116, Wait},
		{"Price Drop (Still WAIT)", 90, Wait},
		{"Cross Below (SELL)", 80, Sell},
		{"Still Below (WAIT)", 70, Wait},
		{"Still Below (WAIT) - #2", 60, Wait},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			signal := strat.OnMarketData(exchange.MarketData{
				Symbol: "BTCUSDT",
				Price:  tt.price,
			})
			if signal.Type != tt.expected {
				t.Errorf("expected %v, got %v for %s", tt.expected, signal.Type, tt.name)
			}
		})
	}
}

func TestSMACrossover_StopLoss(t *testing.T) {
	strat, _ := New("sma_crossover", map[string]interface{}{
		"short_period": 2,
		"long_period":  4,
		"stop_loss_pct": 5.0,
	})

	// Fill data to trigger BUY
	strat.OnMarketData(exchange.MarketData{Price: 100})
	strat.OnMarketData(exchange.MarketData{Price: 101})
	strat.OnMarketData(exchange.MarketData{Price: 102})
	
	// Trigger BUY at 110
	sig := strat.OnMarketData(exchange.MarketData{Price: 110})
	if sig.Type != Buy {
		t.Fatalf("expected BUY, got %v", sig.Type)
	}

	// Price drops to 104.5 (5.01% loss from 110)
	sig = strat.OnMarketData(exchange.MarketData{Price: 104.4})
	if sig.Type != Sell {
		t.Errorf("expected SELL (Stop Loss), got %v", sig.Type)
	}
	if sig.Reason == "" || sig.Reason == "No change" {
		t.Errorf("expected stop loss reason, got %s", sig.Reason)
	}
}

func TestSMACrossover_TakeProfit(t *testing.T) {
	strat, _ := New("sma_crossover", map[string]interface{}{
		"short_period": 2,
		"long_period":  4,
		"take_profit_pct": 10.0,
	})

	// Fill data
	for i := 0; i < 3; i++ {
		strat.OnMarketData(exchange.MarketData{Price: 100})
	}
	
	// Buy at 110
	sigBuy := strat.OnMarketData(exchange.MarketData{Price: 110}) // short > long
	if sigBuy.Type != Buy {
		t.Fatalf("expected BUY at 110, got %v", sigBuy.Type)
	}

	// Price goes to 121 (11% gain)
	sig := strat.OnMarketData(exchange.MarketData{Price: 121.1})
	if sig.Type != Sell {
		t.Errorf("expected SELL (Take Profit) at 121.1, got %v (Reason: %s)", sig.Type, sig.Reason)
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

func TestSMACrossover_TrailingStop(t *testing.T) {
	strat, _ := New("sma_crossover", map[string]interface{}{
		"short_period":      2,
		"long_period":       4,
		"trailing_stop_pct": 2.0,
	})

	// 1. Fill data and trigger BUY at 100
	strat.OnMarketData(exchange.MarketData{Price: 90})
	strat.OnMarketData(exchange.MarketData{Price: 91})
	strat.OnMarketData(exchange.MarketData{Price: 92})
	sig := strat.OnMarketData(exchange.MarketData{Price: 100})
	if sig.Type != Buy {
		t.Fatalf("expected BUY at 100, got %v", sig.Type)
	}

	// 2. Price goes up to 200 (Trailing Stop should follow)
	strat.OnMarketData(exchange.MarketData{Price: 150})
	strat.OnMarketData(exchange.MarketData{Price: 200}) // New Peak

	// 3. Price drops to 197 (1.5% drop - Should NOT trigger yet)
	sig = strat.OnMarketData(exchange.MarketData{Price: 197})
	if sig.Type == Sell {
		t.Errorf("expected WAIT at 197 (1.5%% drop), got SELL. Reason: %s", sig.Reason)
	}

	// 4. Price drops to 195 (2.5% drop - Should TRIGGER Sell)
	sig = strat.OnMarketData(exchange.MarketData{Price: 195})
	if sig.Type != Sell {
		t.Errorf("expected SELL at 195 (2.5%% drop), got %v", sig.Type)
	}
	if sig.Reason == "" || sig.Reason == "No change" {
		t.Errorf("expected trailing stop reason, got %s", sig.Reason)
	}
}

func TestSMACrossover_TrendFilter(t *testing.T) {
	strat, _ := New("sma_crossover", map[string]interface{}{
		"short_period": 2,
		"long_period":  4,
		"trend_period": 10,
	})

	// 1. Create a "Bearish" Trend (Price < EMA)
	// Fill 10 data points starting high and going low
	for i := 200; i > 190; i-- {
		strat.OnMarketData(exchange.MarketData{Price: float64(i)})
	}

	// 2. Trigger a "Bullish Crossover" at a low price (e.g. 100)
	// But Trend EMA will be around ~195
	strat.OnMarketData(exchange.MarketData{Price: 98})
	strat.OnMarketData(exchange.MarketData{Price: 99})
	
	sig := strat.OnMarketData(exchange.MarketData{Price: 105}) // short (102) > long (100)
	
	// Crossover happens, but Price (105) < Trend EMA (~190+)
	if sig.Type == Buy {
		t.Errorf("expected signal to be SUPPRESSED by trend filter, but got BUY. Reason: %s", sig.Reason)
	}
	
	if sig.Type != Wait {
		t.Errorf("expected WAIT (suppressed), got %v", sig.Type)
	}

	// 3. Move Price ABOVE Trend EMA to verify it works when trend is bullish
	for i := 0; i < 20; i++ {
		strat.OnMarketData(exchange.MarketData{Price: 300})
	}
	// Force a Sell signal first to reset lastSignal
	strat.OnMarketData(exchange.MarketData{Price: 50})
	strat.OnMarketData(exchange.MarketData{Price: 40})
	
	// Now Crossover UP while price (310) > Trend EMA (~300)
	strat.OnMarketData(exchange.MarketData{Price: 305})
	sig = strat.OnMarketData(exchange.MarketData{Price: 310})
	
	if sig.Type != Buy {
		t.Errorf("expected BUY when above trend, got %v. Reason: %s", sig.Type, sig.Reason)
	}
}
