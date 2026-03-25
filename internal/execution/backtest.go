package execution

import (
	"context"
	"fmt"
	"niceboy/internal/exchange"
	"niceboy/internal/strategy"
	"time"
)

type BacktestResult struct {
	Symbol          string
	StrategyName    string
	TotalReturn     float64
	TotalReturnPct  float64
	WinRate         float64
	MaxDrawdown     float64
	Trades          []BacktestTrade
	TotalTrades     int
	WinningTrades   int
	LosingTrades    int
	InitialBalance  float64
	FinalBalance    float64
	Duration        time.Duration
}

type BacktestTrade struct {
	Side      string
	EntryTime time.Time
	ExitTime  time.Time
	EntryPrice float64
	ExitPrice  float64
	Profit     float64
	ProfitPct  float64
	Reason     string
}

func RunBacktest(ctx context.Context, exch exchange.Exchange, strat strategy.Strategy, symbol string, interval string, limit int) (*BacktestResult, error) {
	// 1. Fetch Historical Klines
	fmt.Printf("Fetching %d historical candles (%s) for %s...\n", limit, interval, symbol)
	klines, err := exch.GetKlines(ctx, symbol, interval, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch backtest data: %w", err)
	}

	if len(klines) == 0 {
		return nil, fmt.Errorf("no historical data found for backtest")
	}

	result := &BacktestResult{
		Symbol:       symbol,
		StrategyName: strat.GetName(),
		InitialBalance: 10000.0, // Base simulation units (e.g., USDT)
		Trades:       []BacktestTrade{},
	}

	startTime := time.Unix(klines[0].StartTime/1000, 0)
	endTime := time.Unix(klines[len(klines)-1].EndTime/1000, 0)
	result.Duration = endTime.Sub(startTime)

	// 2. Simulation Loop
	var activeTrade *BacktestTrade
	currentBalance := result.InitialBalance
	highWaterMark := currentBalance

	for i, k := range klines {
		// Feed candle as market data first for mid-candle SL checks
		if activeTrade != nil {
			// Real-time SL simulation based on Low price
			slSignal := strat.OnMarketData(exchange.MarketData{
				Price: k.Low,
				Time: k.StartTime,
			})
			if slSignal.Type == strategy.Sell {
				activeTrade.ExitPrice = k.Low
				activeTrade.ExitTime = time.Unix(k.StartTime/1000, 0)
				activeTrade.Profit = activeTrade.ExitPrice - activeTrade.EntryPrice
				activeTrade.ProfitPct = (activeTrade.Profit / activeTrade.EntryPrice) * 100
				activeTrade.Reason = "SL Hit (Mid-Candle)"
				
				result.Trades = append(result.Trades, *activeTrade)
				currentBalance += activeTrade.Profit * 0.1 // Simulated 0.1 qty for scale
				activeTrade = nil
			}
		}

		// Process finalized candle
		sig := strat.OnKline(k)
		
		if sig.Type == strategy.Buy && activeTrade == nil {
			activeTrade = &BacktestTrade{
				Side: "BUY",
				EntryPrice: k.Close,
				EntryTime: time.Unix(k.EndTime/1000, 0),
			}
		} else if sig.Type == strategy.Sell && activeTrade != nil {
			activeTrade.ExitPrice = k.Close
			activeTrade.ExitTime = time.Unix(k.EndTime/1000, 0)
			activeTrade.Profit = activeTrade.ExitPrice - activeTrade.EntryPrice
			activeTrade.ProfitPct = (activeTrade.Profit / activeTrade.EntryPrice) * 100
			activeTrade.Reason = sig.Reason
			
			result.Trades = append(result.Trades, *activeTrade)
			currentBalance += activeTrade.Profit * 0.1 // Simulated 0.1 qty for scale
			activeTrade = nil
		}
		
		// Update Drawdown (Approximate based on closed trades)
		if currentBalance > highWaterMark {
			highWaterMark = currentBalance
		}
		drawdown := (highWaterMark - currentBalance) / highWaterMark * 100
		if drawdown > result.MaxDrawdown {
			result.MaxDrawdown = drawdown
		}
		
		// Progress update
		if i % 100 == 0 {
			fmt.Printf("Processing... %d/%d\n", i, len(klines))
		}
	}

	// 3. Finalize Result
	result.FinalBalance = currentBalance
	result.TotalReturn = currentBalance - result.InitialBalance
	result.TotalReturnPct = (result.TotalReturn / result.InitialBalance) * 100
	result.TotalTrades = len(result.Trades)
	
	for _, t := range result.Trades {
		if t.Profit > 0 {
			result.WinningTrades++
		} else {
			result.LosingTrades++
		}
	}
	if result.TotalTrades > 0 {
		result.WinRate = (float64(result.WinningTrades) / float64(result.TotalTrades)) * 100
	}

	return result, nil
}
