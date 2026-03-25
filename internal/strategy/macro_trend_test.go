package strategy

import (
	"niceboy/internal/exchange"
	"testing"
)

func TestMacroTrend(t *testing.T) {
	params := map[string]interface{}{
		"short_period": 2,
		"long_period":  5,
		"trend_period": 10,
		"atr_period":   3,
		"atr_multiplier": 1.0,
	}

	strat, err := New("macro_trend", params)
	if err != nil {
		t.Fatalf("Failed to create strategy: %v", err)
	}

	// 1. Mock historical data (Macro Bullish: Price > Trend EMA)
	// For period 10, we need at least 10 candles
	var lastSig Signal
	for i := 1; i <= 15; i++ {
		sig := strat.OnKline(exchange.Kline{
			Close:   float64(100 + i),
			High:    float64(100 + i + 1),
			Low:     float64(100 + i - 1),
			IsFinal: true,
		})
		if sig.Type == Buy {
			lastSig = sig
		}
	}

	if lastSig.Type != Buy {
		t.Errorf("Expected BUY signal during warmup, got none")
	}

	// 2. Test Stop Loss trigger
	stopLossSig := strat.OnMarketData(exchange.MarketData{
		Price: lastSig.StopLoss - 1,
	})

	if stopLossSig.Type != Sell {
		t.Errorf("Expected SELL signal from Stop Loss, got %v", stopLossSig.Type)
	}

	// 3. Test Crossover SELL
	strat.LoadState(map[string]interface{}{
		"inPosition": true,
		"entryPrice": 120.0,
		"lastSignal": float64(Buy),
	})

	// Drop price fast to cause EMA crossover
	lastSig = Signal{Type: Wait}
	for i := 0; i < 20; i++ {
		sig := strat.OnKline(exchange.Kline{
			Close:   50,
			High:    51,
			Low:     49,
			IsFinal: true,
		})
		if sig.Type == Sell {
			lastSig = sig
		}
	}

	if lastSig.Type != Sell {
		t.Errorf("Expected SELL signal from Crossover during drop, got none")
	}
}
