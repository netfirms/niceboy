package strategy

import (
	"fmt"
	"math"
	"niceboy/internal/exchange"
)

func init() {
	Register("macro_trend", func(params map[string]interface{}) (Strategy, error) {
		shortP := 20
		longP := 50
		trendP := 200
		atrP := 14
		atrMult := 2.5
		interval := "4h"

		if val, err := getIntParam(params, "short_period"); err == nil { shortP = val }
		if val, err := getIntParam(params, "long_period"); err == nil { longP = val }
		if val, err := getIntParam(params, "trend_period"); err == nil { trendP = val }
		if val, err := getIntParam(params, "atr_period"); err == nil { atrP = val }
		if val, err := getFloatParam(params, "atr_multiplier"); err == nil { atrMult = val }
		if val, ok := params["interval"].(string); ok { interval = val }

		if shortP >= longP {
			return nil, fmt.Errorf("short_period must be less than long_period")
		}

		return &MacroTrend{
			shortPeriod:   shortP,
			longPeriod:    longP,
			trendPeriod:   trendP,
			atrPeriod:     atrP,
			atrMultiplier: atrMult,
			interval:      interval,
			klines:        []exchange.Kline{},
		}, nil
	})
}

// MacroTrend is a low-frequency trend following strategy
type MacroTrend struct {
	shortPeriod   int
	longPeriod    int
	trendPeriod   int
	atrPeriod     int
	atrMultiplier float64
	interval      string

	klines     []exchange.Kline
	inPosition bool
	entryPrice float64
	stopLoss   float64
	lastSignal SignalType
}

func (s *MacroTrend) GetName() string {
	return "macro_trend"
}

func (s *MacroTrend) GetInterval() string {
	return s.interval
}

func (s *MacroTrend) GetHistoryLimit() int {
	max := s.trendPeriod
	if s.longPeriod > max { max = s.longPeriod }
	if s.atrPeriod > max { max = s.atrPeriod }
	return max + 20
}

func (s *MacroTrend) OnMarketData(data exchange.MarketData) Signal {
	// Real-time stop loss check
	if s.inPosition && s.stopLoss > 0 {
		if data.Price <= s.stopLoss {
			s.inPosition = false
			s.lastSignal = Sell
			return Signal{
				Type:   Sell,
				Symbol: data.Symbol,
				Price:  data.Price,
				Profit: data.Price - s.entryPrice,
				Reason: fmt.Sprintf("ATR STOP LOSS hit at %.2f (Stop: %.2f)", data.Price, s.stopLoss),
			}
		}
	}
	return Signal{Type: Wait}
}

func (s *MacroTrend) OnKline(kline exchange.Kline) Signal {
	if !kline.IsFinal {
		return Signal{Type: Wait}
	}

	s.klines = append(s.klines, kline)
	limit := s.GetHistoryLimit()
	if len(s.klines) > limit {
		s.klines = s.klines[len(s.klines)-limit:]
	}

	if len(s.klines) < s.trendPeriod {
		return Signal{
			Type:   Wait,
			Symbol: kline.Symbol,
			Reason: fmt.Sprintf("Collecting historical data (%d/%d)", len(s.klines), s.trendPeriod),
		}
	}

	// Calculate Indicators
	closes := s.getCloses()
	shortEMA := s.calculateEMA(closes, s.shortPeriod)
	longEMA := s.calculateEMA(closes, s.longPeriod)
	trendEMA := s.calculateEMA(closes, s.trendPeriod)
	atr := s.calculateATR(s.atrPeriod)

	sig := Signal{
		Symbol: kline.Symbol,
		Price:  kline.Close,
	}

	// Strategy Logic:
	// 1. MACRO FILTER (Price > Trend EMA)
	if kline.Close < trendEMA {
		return Signal{Type: Wait, Symbol: kline.Symbol, Price: kline.Close, Reason: fmt.Sprintf("Wait (Price %.2f < EMA%d %.2f)", kline.Close, s.trendPeriod, trendEMA)}
	}

	// 2. MOMENTUM FILTER (Short EMA > Long EMA)
	if shortEMA < longEMA {
		if s.inPosition {
			// Exit when momentum flips
			s.inPosition = false
			s.lastSignal = Sell
			sig.Type = Sell
			sig.Profit = kline.Close - s.entryPrice
			sig.Reason = fmt.Sprintf("MacroTrend SELL: EMA%d (%.2f) crossed below EMA%d (%.2f)", s.shortPeriod, shortEMA, s.longPeriod, longEMA)
			return sig
		}
		return Signal{Type: Wait, Symbol: kline.Symbol, Price: kline.Close, Reason: fmt.Sprintf("Wait (EMA%d %.2f < EMA%d %.2f)", s.shortPeriod, shortEMA, s.longPeriod, longEMA)}
	}

	// 3. BULLISH ALIGNMENT
	if !s.inPosition && s.lastSignal != Buy {
		s.inPosition = true
		s.entryPrice = kline.Close
		s.stopLoss = kline.Close - (atr * s.atrMultiplier)
		s.lastSignal = Buy
		sig.Type = Buy
		sig.StopLoss = s.stopLoss
		sig.Reason = fmt.Sprintf("MacroTrend BUY: Price > EMA%d and EMA%d > EMA%d (ATR: %.2f, SL: %.2f)", s.trendPeriod, s.shortPeriod, s.longPeriod, atr, s.stopLoss)
		return sig
	}

	if s.inPosition {
		return Signal{Type: Wait, Symbol: kline.Symbol, Price: kline.Close, Reason: "Position Active (Bullish alignment maintained)"}
	}

	return Signal{Type: Wait, Symbol: kline.Symbol, Price: kline.Close, Reason: "Wait (Waiting for trend confirmation)"}
}

func (s *MacroTrend) getCloses() []float64 {
	closes := make([]float64, len(s.klines))
	for i, k := range s.klines {
		closes[i] = k.Close
	}
	return closes
}

func (s *MacroTrend) calculateEMA(data []float64, period int) float64 {
	if period <= 0 || len(data) < period { return 0 }
	alpha := 2.0 / (float64(period) + 1.0)
	
	// Better EMA initialization: Start with the first data point
	ema := data[0]

	for i := 1; i < len(data); i++ {
		ema = (data[i] * alpha) + (ema * (1.0 - alpha))
	}
	return ema
}

func (s *MacroTrend) calculateATR(period int) float64 {
	if period <= 0 || len(s.klines) < period+1 { return 0 }
	
	trSum := 0.0
	for i := len(s.klines) - period; i < len(s.klines); i++ {
		k := s.klines[i]
		prevK := s.klines[i-1]
		
		tr1 := k.High - k.Low
		tr2 := math.Abs(k.High - prevK.Close)
		tr3 := math.Abs(k.Low - prevK.Close)
		
		tr := math.Max(tr1, math.Max(tr2, tr3))
		trSum += tr
	}
	return trSum / float64(period)
}

func (s *MacroTrend) GetState() map[string]interface{} {
	return map[string]interface{}{
		"inPosition": s.inPosition,
		"entryPrice": s.entryPrice,
		"stopLoss":   s.stopLoss,
		"lastSignal": int(s.lastSignal),
	}
}

func (s *MacroTrend) LoadState(state map[string]interface{}) {
	if val, ok := state["inPosition"].(bool); ok { s.inPosition = val }
	if val, ok := state["entryPrice"].(float64); ok { s.entryPrice = val }
	if val, ok := state["stopLoss"].(float64); ok { s.stopLoss = val }
	if val, ok := state["lastSignal"].(float64); ok {
		s.lastSignal = SignalType(int(val))
	} else if val, ok := state["lastSignal"].(int); ok {
		s.lastSignal = SignalType(val)
	}
}
