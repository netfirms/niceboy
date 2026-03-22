package strategy

import (
	"fmt"
	"niceboy/internal/exchange"
)

func init() {
	Register("sma_crossover", func() Strategy {
		return &SMACrossover{
			shortPeriod: 5,
			longPeriod:  10,
			prices:      []float64{},
		}
	})
}

// SMACrossover is a sample strategy that buys when short SMA crosses above long SMA
type SMACrossover struct {
	shortPeriod int
	longPeriod  int
	prices      []float64
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

	fmt.Printf("[Strategy] Short SMA: %.2f, Long SMA: %.2f\n", shortSMA, longSMA)

	if shortSMA > longSMA {
		return Signal{Type: Buy, Symbol: data.Symbol, Price: data.Price, Reason: "Short SMA crossed above Long SMA"}
	} else if shortSMA < longSMA {
		return Signal{Type: Sell, Symbol: data.Symbol, Price: data.Price, Reason: "Short SMA crossed below Long SMA"}
	}

	return Signal{Type: Wait, Symbol: data.Symbol, Reason: "No crossover detected"}
}

func (s *SMACrossover) calculateSMA(period int) float64 {
	sum := 0.0
	subset := s.prices[len(s.prices)-period:]
	for _, p := range subset {
		sum += p
	}
	return sum / float64(period)
}
