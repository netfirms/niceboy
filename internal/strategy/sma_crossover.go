package strategy

import (
	"fmt"
	"niceboy/internal/exchange"
)

func init() {
	Register("sma_crossover", func(params map[string]interface{}) (Strategy, error) {
		shortP := 5
		longP := 10
		sl := 0.0 // Default no stop loss
		tp := 0.0 // Default no take profit

		if val, err := getIntParam(params, "short_period"); err == nil {
			shortP = val
		}
		if val, err := getIntParam(params, "long_period"); err == nil {
			longP = val
		}
		if val, err := getFloatParam(params, "stop_loss_pct"); err == nil {
			sl = val
		}
		if val, err := getFloatParam(params, "take_profit_pct"); err == nil {
			tp = val
		}

		ts := 0.0
		if val, err := getFloatParam(params, "trailing_stop_pct"); err == nil {
			ts = val
		}

		trendP := 0 // 0 means disabled
		if val, err := getIntParam(params, "trend_period"); err == nil {
			trendP = val
		}

		if shortP >= longP {
			return nil, fmt.Errorf("short_period must be less than long_period")
		}

		maxBuffer := longP
		if trendP > maxBuffer {
			maxBuffer = trendP
		}

		return &SMACrossover{
			shortPeriod:     shortP,
			longPeriod:      longP,
			stopLossPct:     sl,
			takeProfitPct:   tp,
			trailingStopPct: ts,
			trendPeriod:     trendP,
			prices:          []float64{},
			maxBuffer:       maxBuffer,
		}, nil
	})
}

// Helper to extract int from interface
func getIntParam(params map[string]interface{}, key string) (int, error) {
	val, ok := params[key]
	if !ok {
		return 0, fmt.Errorf("param not found")
	}
	switch v := val.(type) {
	case int:
		return v, nil
	case float64:
		return int(v), nil
	default:
		return 0, fmt.Errorf("invalid type for %s", key)
	}
}

func getFloatParam(params map[string]interface{}, key string) (float64, error) {
	val, ok := params[key]
	if !ok {
		return 0, fmt.Errorf("param not found")
	}
	switch v := val.(type) {
	case float64:
		return v, nil
	case int:
		return float64(v), nil
	default:
		return 0, fmt.Errorf("invalid type for %s", key)
	}
}

// SMACrossover is a sample strategy that buys when short SMA crosses above long SMA
type SMACrossover struct {
	shortPeriod     int
	longPeriod      int
	stopLossPct     float64
	takeProfitPct   float64
	trailingStopPct float64
	trendPeriod     int

	prices       []float64
	maxBuffer    int
	entryPrice   float64
	highestPrice float64
	inPosition   bool
	lastSignal   SignalType
}

func (s *SMACrossover) GetName() string {
	return "sma_crossover"
}

func (s *SMACrossover) OnMarketData(data exchange.MarketData) Signal {
	s.prices = append(s.prices, data.Price)
	if len(s.prices) > s.maxBuffer {
		s.prices = s.prices[1:]
	}

	// Wait for enough data for the LONGEST period (usually trend_period)
	required := s.longPeriod
	if s.trendPeriod > required {
		required = s.trendPeriod
	}

	if len(s.prices) < required {
		return Signal{Type: Wait, Symbol: data.Symbol, Reason: fmt.Sprintf("Collecting data (%d/%d)...", len(s.prices), required)}
	}

	shortSMA := s.calculateSMA(s.shortPeriod)
	longSMA := s.calculateSMA(s.longPeriod)
	
	var trendEMA float64
	if s.trendPeriod > 0 {
		trendEMA = s.calculateEMA(s.trendPeriod)
	}

	// 1. Check Exit Conditions (SL/TP/Trailing) if in position
	if s.inPosition {
		// Update highest price for trailing stop
		if data.Price > s.highestPrice {
			s.highestPrice = data.Price
		}

		// Trailing Stop Loss
		if s.trailingStopPct > 0 {
			threshold := s.highestPrice * (1.0 - s.trailingStopPct/100.0)
			if data.Price <= threshold {
				profit := data.Price - s.entryPrice
				s.inPosition = false
				s.lastSignal = Sell
				return Signal{Type: Sell, Symbol: data.Symbol, Price: data.Price, Profit: profit, Reason: fmt.Sprintf("TRAILING STOP hit at %.2f (Peak: %.2f)", data.Price, s.highestPrice)}
			}
		}

		// Fixed Stop Loss
		if s.stopLossPct > 0 {
			threshold := s.entryPrice * (1.0 - s.stopLossPct/100.0)
			if data.Price <= threshold {
				profit := data.Price - s.entryPrice
				s.inPosition = false
				s.lastSignal = Sell
				return Signal{Type: Sell, Symbol: data.Symbol, Price: data.Price, Profit: profit, Reason: fmt.Sprintf("STOP LOSS hit at %.2f (Entry: %.2f)", data.Price, s.entryPrice)}
			}
		}

		// Take Profit
		if s.takeProfitPct > 0 {
			threshold := s.entryPrice * (1.0 + s.takeProfitPct/100.0)
			if data.Price >= threshold {
				profit := data.Price - s.entryPrice
				s.inPosition = false
				s.lastSignal = Sell
				return Signal{Type: Sell, Symbol: data.Symbol, Price: data.Price, Profit: profit, Reason: fmt.Sprintf("TAKE PROFIT hit at %.2f (Entry: %.2f)", data.Price, s.entryPrice)}
			}
		}
	}

	// 2. Check Signals
	if shortSMA > longSMA {
		if !s.inPosition && s.lastSignal != Buy {
			// Trend Filter Confirmation
			if s.trendPeriod > 0 && data.Price < trendEMA {
				return Signal{Type: Wait, Symbol: data.Symbol, Reason: "SMA cross UP suppressed by Bearish Trend (Price < EMA)"}
			}

			s.inPosition = true
			s.entryPrice = data.Price
			s.highestPrice = data.Price // Reset peak
			s.lastSignal = Buy
			return Signal{Type: Buy, Symbol: data.Symbol, Price: data.Price, Reason: "Short SMA crossed above Long SMA (Trend Confirmed)"}
		}
	} else if shortSMA < longSMA {
		if s.inPosition && s.lastSignal != Sell {
			profit := data.Price - s.entryPrice
			s.inPosition = false
			s.lastSignal = Sell
			return Signal{Type: Sell, Symbol: data.Symbol, Price: data.Price, Profit: profit, Reason: "Short SMA crossed below Long SMA"}
		}
	}

	return Signal{Type: Wait, Symbol: data.Symbol, Reason: "No change"}
}

func (s *SMACrossover) GetState() map[string]interface{} {
	return map[string]interface{}{
		"entry_price":   s.entryPrice,
		"highest_price": s.highestPrice,
		"in_position":   s.inPosition,
		"last_signal":   int(s.lastSignal),
	}
}

func (s *SMACrossover) LoadState(state map[string]interface{}) {
	if val, ok := state["entry_price"].(float64); ok {
		s.entryPrice = val
	}
	if val, ok := state["highest_price"].(float64); ok {
		s.highestPrice = val
	}
	if val, ok := state["in_position"].(bool); ok {
		s.inPosition = val
	}
	if val, ok := state["last_signal"].(float64); ok {
		s.lastSignal = SignalType(int(val))
	} else if val, ok := state["last_signal"].(int); ok {
		s.lastSignal = SignalType(val)
	}
}

func (s *SMACrossover) calculateSMA(period int) float64 {
	sum := 0.0
	subset := s.prices[len(s.prices)-period:]
	for _, p := range subset {
		sum += p
	}
	return sum / float64(period)
}

func (s *SMACrossover) calculateEMA(period int) float64 {
	alpha := 2.0 / (float64(period) + 1.0)
	ema := s.prices[len(s.prices)-period] // Start with SMA equivalent or first value
	
	// For better accuracy, we'd want a longer history, but let's approximate
	// by iterating through the available period buffer.
	subset := s.prices[len(s.prices)-period:]
	for _, p := range subset {
		ema = (p * alpha) + (ema * (1.0 - alpha))
	}
	return ema
}
