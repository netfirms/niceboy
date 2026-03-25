package strategy

import (
	"fmt"
	"niceboy/internal/exchange"
)

func init() {
	Register("candle_sma", func(params map[string]interface{}) (Strategy, error) {
		shortP := 20
		longP := 50
		rsiP := 14
		rsiT := 60.0

		if val, err := getIntParam(params, "short_period"); err == nil { shortP = val }
		if val, err := getIntParam(params, "long_period"); err == nil { longP = val }
		if val, err := getIntParam(params, "rsi_period"); err == nil { rsiP = val }
		if val, err := getFloatParam(params, "rsi_threshold"); err == nil { rsiT = val }

		if shortP >= longP {
			return nil, fmt.Errorf("short_period must be less than long_period")
		}

		return &CandleSMA{
			shortPeriod:  shortP,
			longPeriod:   longP,
			rsiPeriod:    rsiP,
			rsiThreshold: rsiT,
			closes:       []float64{},
		}, nil
	})
}

// CandleSMA analyzes 15-minute candlesticks
type CandleSMA struct {
	shortPeriod  int
	longPeriod   int
	rsiPeriod    int
	rsiThreshold float64

	closes       []float64
	inPosition   bool
	entryPrice   float64
	lastSignal   SignalType
}

func (s *CandleSMA) GetName() string {
	return "candle_sma"
}

func (s *CandleSMA) OnMarketData(data exchange.MarketData) Signal {
	// We primarily use OnKline, but we could use this for real-time stop loss
	return Signal{Type: Wait}
}

func (s *CandleSMA) OnKline(kline exchange.Kline) Signal {
	// Only analyze when a candle is finalized, but report status for in-progress ones
	if !kline.IsFinal {
		return Signal{
			Type:   Wait,
			Symbol: kline.Symbol,
			Reason: fmt.Sprintf("Waiting for %s candle to close...", kline.Interval),
		}
	}

	s.closes = append(s.closes, kline.Close)
	if len(s.closes) > 200 {
		s.closes = s.closes[1:]
	}

	if len(s.closes) < s.longPeriod {
		return Signal{
			Type:   Wait,
			Symbol: kline.Symbol,
			Reason: fmt.Sprintf("Collecting candles (%d/%d)", len(s.closes), s.longPeriod),
		}
	}

	shortSMA := s.calculateSMA(s.shortPeriod)
	longSMA := s.calculateSMA(s.longPeriod)
	rsi := s.calculateRSI(s.rsiPeriod)

	sig := Signal{
		Symbol: kline.Symbol,
		Price:  kline.Close,
	}

	if shortSMA > longSMA && rsi < s.rsiThreshold {
		if !s.inPosition && s.lastSignal != Buy {
			s.inPosition = true
			s.lastSignal = Buy
			s.entryPrice = kline.Close
			sig.Type = Buy
			sig.Reason = fmt.Sprintf("15M Candle SMA Cross UP (SMA20: %.2f, SMA50: %.2f, RSI: %.1f)", shortSMA, longSMA, rsi)
			return sig
		}
	} else if shortSMA < longSMA {
		if s.inPosition && s.lastSignal != Sell {
			profit := kline.Close - s.entryPrice
			s.inPosition = false
			s.lastSignal = Sell
			sig.Type = Sell
			sig.Profit = profit
			sig.Reason = fmt.Sprintf("15M Candle SMA Cross DOWN (SMA20: %.2f, SMA50: %.2f)", shortSMA, longSMA)
			return sig
		}
	}

	return Signal{Type: Wait}
}

func (s *CandleSMA) calculateSMA(period int) float64 {
	if len(s.closes) < period { return 0 }
	sum := 0.0
	subset := s.closes[len(s.closes)-period:]
	for _, p := range subset { sum += p }
	return sum / float64(period)
}

func (s *CandleSMA) calculateRSI(period int) float64 {
	if period <= 0 || len(s.closes) < period+1 { return 0 }
	var gains, losses float64
	for i := len(s.closes) - period; i < len(s.closes); i++ {
		diff := s.closes[i] - s.closes[i-1]
		if diff > 0 { gains += diff } else { losses -= diff }
	}
	avgGain := gains / float64(period)
	avgLoss := losses / float64(period)
	if avgLoss == 0 { return 100 }
	rs := avgGain / avgLoss
	return 100.0 - (100.0 / (1.0 + rs))
}

func (s *CandleSMA) GetState() map[string]interface{} {
	return map[string]interface{}{
		"inPosition": s.inPosition,
		"entryPrice": s.entryPrice,
		"lastSignal": int(s.lastSignal),
	}
}

func (s *CandleSMA) LoadState(state map[string]interface{}) {
	if val, ok := state["inPosition"].(bool); ok { s.inPosition = val }
	if val, ok := state["entryPrice"].(float64); ok { s.entryPrice = val }
	if val, ok := state["lastSignal"].(float64); ok { s.lastSignal = SignalType(int(val)) }
}

func (s *CandleSMA) GetHistoryLimit() int {
	return s.longPeriod + 10 // Buffer for safety
}

func (s *CandleSMA) GetInterval() string {
	return "15m"
}
