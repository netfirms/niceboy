package strategy

import (
	"fmt"
	"math"
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
	minGapPct       float64
	confirmTicks    int
	rsiPeriod       int
	rsiThreshold    float64
	maxDevPct       float64

	prices              []float64
	maxBuffer           int
	entryPrice          float64
	highestPrice        float64
	inPosition          bool
	lastSignal          SignalType
	currentConfirmTicks int
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
		return Signal{
			Type:   Wait,
			Symbol: data.Symbol,
			Reason: fmt.Sprintf("Collecting data (%d/%d)...", len(s.prices), required),
		}
	}

	shortSMA := s.calculateSMA(s.shortPeriod)
	longSMA := s.calculateSMA(s.longPeriod)

	var trendEMA float64
	if s.trendPeriod > 0 {
		trendEMA = s.calculateEMA(s.trendPeriod)
	}

	// Prepare base signal with cockpit metadata
	sig := Signal{
		Symbol:       data.Symbol,
		Price:        data.Price,
		EntryPrice:   s.entryPrice,
		StopLoss:     0,
		TakeProfit:   0,
		TrailingStop: 0,
	}

	if s.inPosition {
		if s.stopLossPct > 0 {
			sig.StopLoss = s.entryPrice * (1.0 - s.stopLossPct/100.0)
		}
		if s.takeProfitPct > 0 {
			sig.TakeProfit = s.entryPrice * (1.0 + s.takeProfitPct/100.0)
		}
		if s.trailingStopPct > 0 {
			sig.TrailingStop = s.highestPrice * (1.0 - s.trailingStopPct/100.0)
		}
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
				sig.Type = Sell
				sig.Profit = profit
				sig.Reason = fmt.Sprintf("TRAILING STOP hit at %.2f (Peak: %.2f)", data.Price, s.highestPrice)
				return sig
			}
		}

		// Fixed Stop Loss
		if s.stopLossPct > 0 {
			threshold := s.entryPrice * (1.0 - s.stopLossPct/100.0)
			if data.Price <= threshold {
				profit := data.Price - s.entryPrice
				s.inPosition = false
				s.lastSignal = Sell
				sig.Type = Sell
				sig.Profit = profit
				sig.Reason = fmt.Sprintf("STOP LOSS hit at %.2f (Entry: %.2f)", data.Price, s.entryPrice)
				return sig
			}
		}

		// Take Profit
		if s.takeProfitPct > 0 {
			threshold := s.entryPrice * (1.0 + s.takeProfitPct/100.0)
			if data.Price >= threshold {
				profit := data.Price - s.entryPrice
				s.inPosition = false
				s.lastSignal = Sell
				sig.Type = Sell
				sig.Profit = profit
				sig.Reason = fmt.Sprintf("TAKE PROFIT hit at %.2f (Entry: %.2f)", data.Price, s.entryPrice)
				return sig
			}
		}
	}

	// 2. Indicators for confirmation
	rsi := s.calculateRSI(s.rsiPeriod)
	devPct := 0.0
	if longSMA > 0 {
		devPct = ((data.Price - longSMA) / longSMA) * 100.0
	}

	gap := 0.0
	if longSMA > 0 {
		gap = (math.Abs(shortSMA-longSMA) / longSMA) * 100.0
	}

	if shortSMA > longSMA {
		// Potential BUY
		if !s.inPosition && s.lastSignal != Buy {
			// Hysteresis Filter
			if gap < s.minGapPct {
				s.currentConfirmTicks = 0
				sig.Type = Wait
				sig.Reason = fmt.Sprintf("Gap too small (%.4f%% < %.2f%%)", gap, s.minGapPct)
				return sig
			}

			// Confirmation Filter
			s.currentConfirmTicks++
			if s.currentConfirmTicks < s.confirmTicks {
				sig.Type = Wait
				sig.Reason = fmt.Sprintf("Confirming cross UP (%d/%d)...", s.currentConfirmTicks, s.confirmTicks)
				return sig
			}

			// 3. Counter-Trend Filters (RSI & Deviation)
			if s.rsiPeriod > 0 && rsi > s.rsiThreshold {
				sig.Type = Wait
				sig.Reason = fmt.Sprintf("SMA Cross UP ignored: Overbought (RSI: %.1f > %.1f)", rsi, s.rsiThreshold)
				return sig
			}

			if s.maxDevPct > 0 && devPct > s.maxDevPct {
				sig.Type = Wait
				sig.Reason = fmt.Sprintf("SMA Cross UP ignored: Price too far from mean (Dev: %.2f%% > %.2f%%)", devPct, s.maxDevPct)
				return sig
			}

			// Trend Filter Confirmation
			if s.trendPeriod > 0 && data.Price < trendEMA {
				sig.Type = Wait
				sig.Reason = "SMA cross UP suppressed by Bearish Trend (Price < EMA)"
				return sig
			}

			s.inPosition = true
			s.entryPrice = data.Price
			s.highestPrice = data.Price // Reset peak
			s.lastSignal = Buy
			s.currentConfirmTicks = 0 // Reset after trigger

			sig.Type = Buy
			sig.EntryPrice = s.entryPrice
			sig.Reason = fmt.Sprintf("Short SMA crossed above Long SMA (Gap: %.2f%%, RSI: %.1f)", gap, rsi)

			// Recalculate guardrails for the BUY signal
			if s.stopLossPct > 0 {
				sig.StopLoss = s.entryPrice * (1.0 - s.stopLossPct/100.0)
			}
			if s.takeProfitPct > 0 {
				sig.TakeProfit = s.entryPrice * (1.0 + s.takeProfitPct/100.0)
			}
			if s.trailingStopPct > 0 {
				sig.TrailingStop = s.highestPrice * (1.0 - s.trailingStopPct/100.0)
			}

			return sig
		}
		// Reset confirmation if we are already in position or last signal was BUY
		s.currentConfirmTicks = 0
	} else if shortSMA < longSMA {
		// Potential SELL
		if s.inPosition && s.lastSignal != Sell {
			// Hysteresis for EXIT if desired (optional, usually we want to exit fast)
			if gap < s.minGapPct {
				s.currentConfirmTicks = 0
				sig.Type = Wait
				sig.Reason = fmt.Sprintf("Gap too small for exit (%.4f%%)", gap)
				return sig
			}

			// Confirmation for EXIT
			s.currentConfirmTicks++
			if s.currentConfirmTicks < s.confirmTicks {
				sig.Type = Wait
				sig.Reason = fmt.Sprintf("Confirming cross DOWN (%d/%d)...", s.currentConfirmTicks, s.confirmTicks)
				return sig
			}

			profit := data.Price - s.entryPrice
			s.inPosition = false
			s.lastSignal = Sell
			s.currentConfirmTicks = 0 // Reset after trigger

			sig.Type = Sell
			sig.Profit = profit
			sig.Reason = fmt.Sprintf("Short SMA crossed below Long SMA (Gap: %.2f%%)", gap)
			return sig
		}
		s.currentConfirmTicks = 0
	} else {
		s.currentConfirmTicks = 0
	}

	sig.Type = Wait
	sig.Reason = "No change"
	return sig
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

func (s *SMACrossover) calculateRSI(period int) float64 {
	if len(s.prices) <= period {
		return 50.0 // Default to neutral if not enough data
	}

	totalGain := 0.0
	totalLoss := 0.0

	// Use the last 'period' observations
	subset := s.prices[len(s.prices)-(period+1):]
	for i := 1; i < len(subset); i++ {
		diff := subset[i] - subset[i-1]
		if diff > 0 {
			totalGain += diff
		} else {
			totalLoss += -diff
		}
	}

	if totalLoss == 0 {
		return 100.0
	}

	rs := totalGain / totalLoss
	return 100.0 - (100.0 / (1.0 + rs))
}
