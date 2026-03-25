package strategy

import (
	"niceboy/internal/exchange"
	"testing"
)

func TestCandleSMA_OnKline(t *testing.T) {
	strat, _ := New("candle_sma", map[string]interface{}{
		"short_period": 2,
		"long_period":  4,
		"rsi_period":   0,
	})

	// 1. Fill data (4 candles)
	for i := 0; i < 3; i++ {
		sig := strat.OnKline(exchange.Kline{Close: 100, IsFinal: true})
		if sig.Type != Wait {
			t.Errorf("expected WAIT while collecting data, got %v", sig.Type)
		}
	}

	// 2. Trigger BUY
	// SMA2: (100+110)/2 = 105
	// SMA4: (100+100+100+110)/4 = 102.5
	// RSI will be low because it's the first jump
	sig := strat.OnKline(exchange.Kline{Symbol: "BTCUSDT", Close: 110, IsFinal: true})
	if sig.Type != Buy {
		t.Errorf("expected BUY on cross up, got %v. Reason: %s", sig.Type, sig.Reason)
	}

	// 3. Trigger SELL
	// SMA2: (110+90)/2 = 100
	// SMA4: (100+100+110+90)/4 = 100 -> actually equal, let's go lower
	sig = strat.OnKline(exchange.Kline{Symbol: "BTCUSDT", Close: 80, IsFinal: true})
	if sig.Type != Sell {
		t.Errorf("expected SELL on cross down, got %v. Reason: %s", sig.Type, sig.Reason)
	}
}
