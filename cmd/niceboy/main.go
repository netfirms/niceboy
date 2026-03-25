package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"math"
	"niceboy/internal/config"
	"niceboy/internal/database"
	"niceboy/internal/exchange"
	"niceboy/internal/strategy"
	"niceboy/internal/ui"
	"niceboy/internal/execution"
	"strconv"

	"github.com/charmbracelet/bubbletea"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"path/filepath"
	"strings"
	"sync"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	configPath := flag.String("config", "config.yaml", "Path to configuration file")
	showVersion := flag.Bool("version", false, "Show version information")
	backtest := flag.Bool("backtest", false, "Run historical backtest")
	limit := flag.Int("limit", 500, "Number of candles to backtest")
	flag.Parse()

	if *showVersion {
		fmt.Printf("niceboy %s (commit: %s, date: %s)\n", version, commit, date)
		return
	}

	configName := filepath.Base(*configPath)
	configName = strings.TrimSuffix(configName, filepath.Ext(configName))

	// Ensure specialized log directory per config
	logDir := fmt.Sprintf("logs/%s", configName)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		fmt.Printf("Warning: Failed to create log directory: %v\n", err)
	}
	
	finalLogPath := filepath.Join(logDir, "niceboy.log")
	logFile, err := os.OpenFile(finalLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("Warning: Failed to open log file %s: %v\n", finalLogPath, err)
	} else {
		defer logFile.Close()
	}

	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = zerolog.New(logFile).With().Timestamp().Logger()
	log.Info().Msg("⚡ niceboy starting...")

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to load config, starting interactive setup...")
		cfg = &config.Config{}
		if err := cfg.InteractiveSetup(); err != nil {
			log.Fatal().Err(err).Msg("Setup failed")
		}
		cfg.SaveConfig(*configPath)
	} else if cfg.NeedsSetup() {
		if err := cfg.InteractiveSetup(); err != nil {
			log.Fatal().Err(err).Msg("Setup failed")
		}
		cfg.SaveConfig(*configPath)
	}

	// Ensure specialized data directory per config
	dataDir := fmt.Sprintf("data/%s", configName)
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Warn().Err(err).Msg("Failed to create data directory, using root")
		dataDir = "."
	}
	dbPath := filepath.Join(dataDir, "niceboy.db")

	dbStore, err := database.NewSQLiteStore(dbPath)
	if err != nil {
		log.Fatal().Err(err).Str("path", dbPath).Msg("Failed to initialize database")
	}
	defer dbStore.Close()

	var exch exchange.Exchange
	exchCfg := cfg.Exchanges[cfg.ActiveExchange]

	switch cfg.ActiveExchange {
	case "binance":
		exch = exchange.NewBinanceExchange(exchCfg.Key, exchCfg.Secret)
	case "bitkub":
		exch = exchange.NewBitkubExchange(exchCfg.Key, exchCfg.Secret)
	default:
		log.Fatal().Str("exchange", cfg.ActiveExchange).Msg("Unsupported exchange")
	}

	if exch != nil {
		symbol := exchCfg.Symbol
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		symbolInfo, err := exch.GetSymbolInfo(ctx, symbol)
		if err != nil {
			log.Warn().Err(err).Str("symbol", symbol).Msg("Failed to fetch symbol info, using defaults")
			symbolInfo = exchange.SymbolInfo{
				Symbol: symbol, BasePrecision: 8, QuotePrecision: 2, MinQty: 0.0001,
			}
		}

		strategyName := cfg.Strategy
		if strategyName == "" {
			strategyName = "sma_crossover"
		}

		strat, err := strategy.New(strategyName, cfg.StrategyParameters)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to initialize strategy")
		}

			if stateJSON, err := dbStore.GetState("strategy_state"); err == nil && stateJSON != "" {
				var state map[string]interface{}
				if err := json.Unmarshal([]byte(stateJSON), &state); err == nil {
					strat.LoadState(state)
				}
			}

		if *backtest {
			log.Info().Str("strategy", strategyName).Str("symbol", symbol).Int("limit", *limit).Msg("🚀 Starting historical backtest...")
			res, err := execution.RunBacktest(context.Background(), exch, strat, symbol, cfg.KlineInterval, *limit)
			if err != nil {
				log.Fatal().Err(err).Msg("Backtest failed")
			}

			fmt.Println("\n==================================================")
			fmt.Printf(" BACKTEST RESULTS: %s (%s)\n", strategyName, symbol)
			fmt.Println("==================================================")
			fmt.Printf(" Period:       %s to %s (%v)\n", res.Trades[0].EntryTime.Format("2006-01-02"), res.Trades[len(res.Trades)-1].ExitTime.Format("2006-01-02"), res.Duration.Round(time.Hour))
			fmt.Printf(" Total Return: %+.2f USDT (%+.2f%%)\n", res.TotalReturn, res.TotalReturnPct)
			fmt.Printf(" Win Rate:     %.1f%% (%d/%d)\n", res.WinRate, res.WinningTrades, res.TotalTrades)
			fmt.Printf(" Max Drawdown: %.2f%%\n", res.MaxDrawdown)
			fmt.Println("--------------------------------------------------")
			fmt.Printf(" Finished at:  %s\n", time.Now().Format(time.RFC822))
			fmt.Println("==================================================")
			return
		}

		engineCh := make(chan interface{}, 20)
		m := ui.NewModel(cfg.ActiveExchange, symbol, cfg.DryRun, dbStore, strategyName, cfg.StrategyParameters, cfg.OrderQuantity, version, commit, configName, engineCh)
		p := tea.NewProgram(m, tea.WithAltScreen())

		var wg sync.WaitGroup		
		pollTriggerCh := make(chan struct{}, 1)
		
		// Loop 5: Trading Loop
		wg.Add(1)
		go func(cmdCtx context.Context) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					p.Send(ui.AuditMsg(fmt.Sprintf("PANIC: %v", r)))
				}
			}()

			marketDataCh := make(chan exchange.MarketData, 100)
			klineCh := make(chan exchange.Kline, 100)

			err := exch.SubscribePrice(cmdCtx, symbol, marketDataCh)
			if err != nil {
				log.Fatal().Err(err).Msg("Failed to subscribe to market data")
			}

			// Also subscribe to klines if interval is specified
			klineInterval := strat.GetInterval()
			if klineInterval == "" {
				klineInterval = cfg.KlineInterval
			}
			if klineInterval == "" {
				klineInterval = "15m" // Ultimate default
			}
			
			// BOOTSTRAP: Load historical klines to avoid "waiting for data"
			historyLimit := strat.GetHistoryLimit()
			if historyLimit <= 0 { historyLimit = 100 }

			p.Send(ui.AuditMsg(fmt.Sprintf("BOOTSTRAP: Loading %d historical %s klines for %s...", historyLimit, klineInterval, strat.GetName())))
			history, err := exch.GetKlines(cmdCtx, symbol, klineInterval, historyLimit)
			if err == nil {
				for _, k := range history {
					strat.OnKline(k)
				}
				p.Send(ui.AuditMsg(fmt.Sprintf("BOOTSTRAP: Success! Loaded %d historical candles.", len(history))))
			} else {
				p.Send(ui.AuditMsg(fmt.Sprintf("BOOTSTRAP: Warning: Could not load history: %v", err)))
			}

			if err := exch.SubscribeKlines(cmdCtx, symbol, klineInterval, klineCh); err != nil {
				log.Warn().Err(err).Str("interval", klineInterval).Msg("Failed to subscribe to klines (assuming ticker-only exchange)")
			}
			p.Send(ui.AuditMsg(fmt.Sprintf("STREAM: Connected to market data (Ticks + %s Klines)", klineInterval)))

			var lastBalances map[string]float64
			halted := false
			if val, err := dbStore.GetState("bot_halted"); err == nil && val == "true" {
				halted = true
				p.Send(ui.AuditMsg("[SYSTEM] Bot is HALTED. Type 'k' to clear logic (exit/restart)."))
			}

			handleSignal := func(sig strategy.Signal) {
				if sig.Type == strategy.Wait {
					return
				}
				p.Send(ui.SignalMsg(sig))

				if sig.Type == strategy.Buy || sig.Type == strategy.Sell {
					// Check balance before executing
					canTrade := true
					if sig.Type == strategy.Buy {
						quote := lastBalances["USDT"] // Simplified
						if quote < sig.Price*cfg.OrderQuantity {
							p.Send(ui.AuditMsg(fmt.Sprintf("SKIP: Insufficient USDT (%.2f < %.2f)", quote, sig.Price*cfg.OrderQuantity)))
							canTrade = false
						}
					} else {
						base := lastBalances["BTC"] // Simplified
						if base < cfg.OrderQuantity {
							p.Send(ui.AuditMsg(fmt.Sprintf("SKIP: Insufficient BTC (%.4f < %.4f)", base, cfg.OrderQuantity)))
							canTrade = false
						}
					}

					if canTrade {
						executedSignal := strategy.Signal{
							Type:       sig.Type,
							Symbol:     symbol,
							Price:      sig.Price,
							Profit:     sig.Profit,
							Reason:     sig.Reason,
							StopLoss:   sig.StopLoss,
							TakeProfit: sig.TakeProfit,
						}
						executeSignal(cmdCtx, executedSignal, lastBalances, exch, cfg, symbolInfo, dbStore, p, strat)
						select {
						case pollTriggerCh <- struct{}{}:
						default:
						}
					}
				}
			}

			for {
				select {
				case <-cmdCtx.Done():
					return
				case rawMsg := <-engineCh:
					switch msg := rawMsg.(type) {
					case map[string]float64:
						lastBalances = msg
					case ui.ManualTradeMsg:
						// Tactical forced trade
						p.Send(ui.AuditMsg(fmt.Sprintf("[TACTICAL] Executing forced %s...", msg.Side)))
						qty := cfg.OrderQuantity
						price, _ := exch.GetPrice(cmdCtx, symbol)
						if err := exch.ExecuteOrder(cmdCtx, symbol, msg.Side, exchange.Market, qty, price); err == nil {
							p.Send(ui.AuditMsg(fmt.Sprintf("SUCCESS: Tactical %s @ %.2f", msg.Side, price)))
							p.Send(ui.TradeExecutedMsg{})
							select {
							case pollTriggerCh <- struct{}{}:
							default:
							}
						} else {
							p.Send(ui.AuditMsg(fmt.Sprintf("FAIL: Tactical %s failed: %v", msg.Side, err)))
						}
					case ui.KillSwitchMsg:
						halted = true
						dbStore.SaveState("bot_halted", "true")
						p.Send(ui.AuditMsg("🛑 EMERGENCY HALT ACTIVATED. Position clearing logic skipped for safety."))
					}
				case data := <-marketDataCh:
					if halted {
						continue
					}
					p.Send(ui.PriceMsg(data))
					handleSignal(strat.OnMarketData(data))
				case kline := <-klineCh:
					if halted {
						continue
					}
					handleSignal(strat.OnKline(kline))
				}
			}
		}(ctx)

		// Loop 6: Polling Loop (Balances, Orders, Stats)
		wg.Add(1)
		go func(cmdCtx context.Context) {
			defer wg.Done()
			ticker := time.NewTicker(2 * time.Second)
			defer ticker.Stop()

			for {
				select {
				case <-cmdCtx.Done():
					return
				case <-ticker.C:
					// Run Poll
					triggerPoll(cmdCtx, exch, p, engineCh, dbStore, symbol)
				case <-pollTriggerCh:
					// Immediate Refresh
					triggerPoll(cmdCtx, exch, p, engineCh, dbStore, symbol)
				}
			}
		}(ctx)

		if _, err := p.Run(); err != nil {
			log.Fatal().Err(err).Msg("UI Failed")
		}
		cancel()
		wg.Wait()
		log.Info().Msg("Shutdown complete")
	}
}

func executeSignal(ctx context.Context, signal strategy.Signal, balances map[string]float64, exch exchange.Exchange, cfg *config.Config, info exchange.SymbolInfo, db database.Store, p *tea.Program, strat strategy.Strategy) {
	if signal.Type == strategy.Wait { return }
	side := exchange.Buy
	if signal.Type == strategy.Sell { side = exchange.Sell }
	
	quantity := cfg.OrderQuantity
	
	// Safety: Pre-execution balance check
	if balances != nil {
		if side == exchange.Buy {
			quoteAsset := info.QuoteAsset
			cost := quantity * signal.Price
			bal, ok := balances[quoteAsset]
			if !ok {
				p.Send(ui.AuditMsg(fmt.Sprintf("SKIP: Balance for %s not found in cache", quoteAsset)))
				return
			}
			if bal < cost {
				p.Send(ui.AuditMsg(fmt.Sprintf("SKIP: Insufficient %s (Need %.2f, Have %.2f)", quoteAsset, cost, bal)))
				return
			}
		} else {
			baseAsset := info.BaseAsset
			bal, ok := balances[baseAsset]
			if !ok {
				p.Send(ui.AuditMsg(fmt.Sprintf("SKIP: Balance for %s not found in cache", baseAsset)))
				return
			}
			if bal < quantity {
				p.Send(ui.AuditMsg(fmt.Sprintf("SKIP: Insufficient %s (Need %.4f, Have %.4f)", baseAsset, quantity, bal)))
				return
			}
		}
	}

	if cfg.DryRun {
		p.Send(ui.AuditMsg(fmt.Sprintf("[DRY] %s %s order (Reason: %s)", side, info.Symbol, signal.Reason)))
		p.Send(ui.TradeExecutedMsg{})
		db.SaveTrade(database.Trade{
			Symbol: info.Symbol, Side: string(side), Price: signal.Price, Quantity: quantity, Profit: signal.Profit * quantity, Timestamp: time.Now(), Reason: "[DRY] "+signal.Reason,
		})
	} else {
		// Real Execution (Limit order with slippage protection)
		mult := 1.0 + (cfg.SlippagePct / 100.0)
		if side == exchange.Sell { mult = 1.0 - (cfg.SlippagePct / 100.0) }
		lPrice := signal.Price * mult
		
		fmtQty := truncateFloat(quantity, info.BasePrecision)
		fmtPrice := strconv.FormatFloat(lPrice, 'f', info.QuotePrecision, 64)
		fQty, _ := strconv.ParseFloat(fmtQty, 64)
		fPrice, _ := strconv.ParseFloat(fmtPrice, 64)

		if fQty < info.MinQty { return }

		execCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		if err := exch.ExecuteOrder(execCtx, info.Symbol, side, exchange.Limit, fQty, fPrice); err != nil {
			p.Send(ui.AuditMsg(fmt.Sprintf("FAIL: %v", err)))
		} else {
			p.Send(ui.AuditMsg(fmt.Sprintf("SUCCESS: %s @ %s", side, fmtPrice)))
			p.Send(ui.TradeExecutedMsg{})
			db.SaveTrade(database.Trade{
				Symbol: info.Symbol, Side: string(side), Price: fPrice, Quantity: fQty, Profit: signal.Profit * fQty, Timestamp: time.Now(), Reason: signal.Reason,
			})
		}
	}

	state := strat.GetState()
	if bytes, err := json.Marshal(state); err == nil {
		db.SaveState("strategy_state", string(bytes))
	}
}

func truncateFloat(f float64, precision int) string {
	shift := math.Pow(10, float64(precision))
	truncated := math.Trunc(f*shift) / shift
	return strconv.FormatFloat(truncated, 'f', precision, 64)
}

func triggerPoll(cmdCtx context.Context, exch exchange.Exchange, p *tea.Program, engineCh chan<- interface{}, dbStore database.Store, symbol string) {
	fetchCtx, fetchCancel := context.WithTimeout(cmdCtx, 8*time.Second)
	defer fetchCancel()

	balances, err := exch.GetBalances(fetchCtx)
	if err == nil {
		p.Send(ui.BalanceUpdateMsg(balances))
		engineCh <- balances
	} else {
		p.Send(ui.AuditMsg(fmt.Sprintf("[ERROR] Failed to fetch balances: %v", err)))
	}

	orders, err := exch.GetOpenOrders(fetchCtx, symbol)
	if err == nil {
		p.Send(ui.OpenOrdersUpdateMsg(orders))
	} else {
		p.Send(ui.AuditMsg(fmt.Sprintf("[ERROR] Failed to fetch open orders: %v", err)))
	}

	book, err := exch.GetOrderBook(fetchCtx, symbol, 5)
	if err == nil {
		p.Send(ui.OrderBookUpdateMsg(book))
	} else {
		p.Send(ui.AuditMsg(fmt.Sprintf("[ERROR] Failed to fetch order book: %v", err)))
	}

	pulse := make(map[string]float64)
	btcP, err := exch.GetPrice(fetchCtx, "BTCUSDT")
	if err == nil {
		pulse["BTC"] = btcP
		p.Send(ui.MarketPulseMsg(pulse))
	}

	stats, err := dbStore.GetStats()
	if err == nil {
		p.Send(ui.StatsUpdateMsg(stats))
	}
}
