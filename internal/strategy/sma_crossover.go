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
		if val, ok := params["stop_loss_pct"].(float64); ok {
			sl = val
		}
		if val, ok := params["take_profit_pct"].(float64); ok {
			tp = val
		}

		if shortP >= longP {
			return nil, fmt.Errorf("short_period must be less than long_period")
		}

		return &SMACrossover{
			shortPeriod:   shortP,
			longPeriod:    longP,
			stopLossPct:   sl,
			takeProfitPct: tp,
			prices:        []float64{},
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

// SMACrossover is a sample strategy that buys when short SMA crosses above long SMA
type SMACrossover struct {
	shortPeriod   int
	longPeriod    int
	stopLossPct   float64
	takeProfitPct float64
	
	prices      []float64
	entryPrice  float64
	inPosition  bool
	lastSignal  SignalType
}

func (s *SMACrossover) GetName() string {
	return "sma_crossover"
}

func (s *SMACrossover) OnMarketData(data exchange.MarketData) Signal {
	s.prices = append(s.prices, data.Price)
	if len(s.prices) > s.longPeriod {
		s.prices = s.prices[1:]
	}

	if len(s.prices) < s.longPeriod {
		return Signal{Type: Wait, Symbol: data.Symbol, Reason: "Collecting data..."}
	}

	shortSMA := s.calculateSMA(s.shortPeriod)
	longSMA := s.calculateSMA(s.longPeriod)

	// 1. Check Exit Conditions (SL/TP) if in position
	if s.inPosition {
		if s.stopLossPct > 0 {
			threshold := s.entryPrice * (1.0 - s.stopLossPct/100.0)
			if data.Price <= threshold {
				s.inPosition = false
				s.lastSignal = Sell
				return Signal{Type: Sell, Symbol: data.Symbol, Price: data.Price, Reason: fmt.Sprintf("STOP LOSS hit at %.2f (Entry: %.2f)", data.Price, s.entryPrice)}
			}
		}
		if s.takeProfitPct > 0 {
			threshold := s.entryPrice * (1.0 + s.takeProfitPct/100.0)
			if data.Price >= threshold {
				s.inPosition = false
				s.lastSignal = Sell
				return Signal{Type: Sell, Symbol: data.Symbol, Price: data.Price, Reason: fmt.Sprintf("TAKE PROFIT hit at %.2f (Entry: %.2f)", data.Price, s.entryPrice)}
			}
		}
	}

	// 2. Check Crossover logic
	if shortSMA > longSMA {
		if !s.inPosition && s.lastSignal != Buy {
			s.inPosition = true
			s.entryPrice = data.Price
			s.lastSignal = Buy
			return Signal{Type: Buy, Symbol: data.Symbol, Price: data.Price, Reason: "Short SMA crossed above Long SMA"}
		}
	} else if shortSMA < longSMA {
		if s.inPosition && s.lastSignal != Sell {
			s.inPosition = false
			s.lastSignal = Sell
			return Signal{Type: Sell, Symbol: data.Symbol, Price: data.Price, Reason: "Short SMA crossed below Long SMA"}
		}
	}

	return Signal{Type: Wait, Symbol: data.Symbol, Reason: "No change"}
}

func (s *SMACrossover) calculateSMA(period int) float64 {
	sum := 0.0
	subset := s.prices[len(s.prices)-period:]
	for _, p := range subset {
		sum += p
	}
	return sum / float64(period)
}
