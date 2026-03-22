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
	"strconv"

	"github.com/charmbracelet/bubbletea"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"sync"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	configPath := flag.String("config", "config.yaml", "Path to configuration file")
	logPath := flag.String("log", "niceboy.log", "Path to log file")
	showVersion := flag.Bool("version", false, "Show version information")
	flag.Parse()

	if *showVersion {
		fmt.Printf("niceboy %s (commit: %s, date: %s)\n", version, commit, date)
		return
	}

	logFile, err := os.OpenFile(*logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("Warning: Failed to open log file %s: %v\n", *logPath, err)
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

	dbStore, err := database.NewSQLiteStore(cfg.DatabasePath)
	if err != nil {
		log.Fatal().Err(err).Str("path", cfg.DatabasePath).Msg("Failed to initialize database")
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

		engineCh := make(chan interface{}, 20)
		m := ui.NewModel(cfg.ActiveExchange, symbol, cfg.DryRun, dbStore, strategyName, cfg.StrategyParameters, cfg.OrderQuantity, version, commit, engineCh)
		p := tea.NewProgram(m, tea.WithAltScreen())

		var wg sync.WaitGroup

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
			err := exch.SubscribePrice(cmdCtx, symbol, marketDataCh)
			if err != nil {
				log.Fatal().Err(err).Msg("Failed to subscribe to market data")
			}
			p.Send(ui.AuditMsg("STREAM: Connected to market data"))

			var lastBalances map[string]float64
			halted := false
			if val, err := dbStore.GetState("bot_halted"); err == nil && val == "true" {
				halted = true
				p.Send(ui.AuditMsg("[SYSTEM] Bot is HALTED. Type 'k' to clear logic (exit/restart)."))
			}

			for {
				select {
				case <-cmdCtx.Done():
					return
				case rawMsg := <-engineCh:
					switch msg := rawMsg.(type) {
					case map[string]float64:
						lastBalances = msg
					case ui.KillSwitchMsg:
						halted = true
						dbStore.SaveState("bot_halted", "true")
						p.Send(ui.AuditMsg("[EMERGENCY] Global halt activated."))
					case ui.ManualTradeMsg:
						p.Send(ui.AuditMsg(fmt.Sprintf("[TACTICAL] Executing manual %s order...", msg.Side)))
						// Manual trades trigger immediate logic bypass
						// Fake a signal to reuse execution logic
						mSignal := strategy.Signal{Type: strategy.Wait, Symbol: symbol, Reason: "User Manual"}
						if msg.Side == exchange.Buy { mSignal.Type = strategy.Buy } else { mSignal.Type = strategy.Sell }
						executeSignal(cmdCtx, mSignal, lastBalances, exch, cfg, symbolInfo, dbStore, p, strat)
					}
				case md, ok := <-marketDataCh:
					if !ok { return }
					p.Send(ui.PriceMsg(md))
					if halted { continue }
					signal := strat.OnMarketData(md)
					if signal.Type != strategy.Wait {
						p.Send(ui.SignalMsg(signal))
						executeSignal(cmdCtx, signal, lastBalances, exch, cfg, symbolInfo, dbStore, p, strat)
					}
				}
			}
		}(ctx)

		// Loop 6: Polling Loop
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
					fetchCtx, fetchCancel := context.WithTimeout(cmdCtx, 8*time.Second)
					balances, err := exch.GetBalances(fetchCtx)
					if err == nil {
						p.Send(ui.BalanceUpdateMsg(balances))
						engineCh <- balances
					}
					orders, _ := exch.GetOpenOrders(fetchCtx, symbol)
					p.Send(ui.OpenOrdersUpdateMsg(orders))
					book, _ := exch.GetOrderBook(fetchCtx, symbol, 5)
					p.Send(ui.OrderBookUpdateMsg(book))
					pulse := make(map[string]float64)
					btcP, _ := exch.GetPrice(fetchCtx, "BTCUSDT")
					pulse["BTC"] = btcP
					p.Send(ui.MarketPulseMsg(pulse))
					stats, _ := dbStore.GetStats()
					p.Send(ui.StatsUpdateMsg(stats))
					fetchCancel()
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
			cost := quantity * signal.Price
			if balances["USDT"] < cost {
				p.Send(ui.AuditMsg(fmt.Sprintf("SKIP: Insufficient USDT (Need %.2f, Have %.2f)", cost, balances["USDT"])))
				return
			}
		} else {
			if balances[info.BaseAsset] < quantity {
				p.Send(ui.AuditMsg(fmt.Sprintf("SKIP: Insufficient %s (Need %.4f, Have %.4f)", info.BaseAsset, quantity, balances[info.BaseAsset])))
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
