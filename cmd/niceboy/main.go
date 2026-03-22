package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"niceboy/internal/config"
	"niceboy/internal/database"
	"niceboy/internal/exchange"
	"niceboy/internal/strategy"
	"niceboy/internal/ui"
	"strconv"
	"math"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	// Parse CLI flags for multi-instance support
	configPath := flag.String("config", "config.yaml", "Path to configuration file")
	logPath := flag.String("log", "niceboy.log", "Path to log file")
	showVersion := flag.Bool("version", false, "Show version information")
	flag.Parse()

	if *showVersion {
		fmt.Printf("niceboy %s (commit: %s, date: %s)\n", version, commit, date)
		return
	}

	// Initialize structured logging (File only to prevent TUI rendering corruption)
	logFile, err := os.OpenFile(*logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("Warning: Failed to open log file %s: %v\n", *logPath, err)
	} else {
		defer logFile.Close()
	}

	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = zerolog.New(logFile).With().Timestamp().Logger()

	log.Info().Msg("⚡ niceboy starting...")

	// 1. Load Configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to load config, starting interactive setup...")
		cfg = &config.Config{} // Start empty
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

	log.Info().Str("exchange", cfg.ActiveExchange).Msg("Configuration loaded")

	// 2. Initialize Database
	dbStore, err := database.NewSQLiteStore(cfg.DatabasePath)
	if err != nil {
		log.Fatal().Err(err).Str("path", cfg.DatabasePath).Msg("Failed to initialize database")
	}
	defer dbStore.Close()

	// 2. Initialize Exchange Integration
	var exch exchange.Exchange
	exchCfg := cfg.Exchanges[cfg.ActiveExchange] // Validated in config.LoadConfig

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

		// Global Context for Graceful Shutdown
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Fetch Symbol Metadata for Precision and Limits
		symbolInfo, err := exch.GetSymbolInfo(ctx, symbol)
		if err != nil {
			log.Warn().Err(err).Str("symbol", symbol).Msg("Failed to fetch symbol info, using defaults")
			symbolInfo = exchange.SymbolInfo{
				Symbol: symbol,
				BasePrecision: 8,
				QuotePrecision: 2,
				MinQty: 0.0001,
			}
		}

		// 3. Initialize Strategy
		strategyName := cfg.Strategy
		if strategyName == "" {
			strategyName = "sma_crossover" // Fallback
		}
		
		strat, err := strategy.New(strategyName, cfg.StrategyParameters)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to initialize strategy")
		}

		// Load Strategy State from DB
		if stateJSON, err := dbStore.GetState("strategy_state"); err == nil && stateJSON != "" {
			var state map[string]interface{}
			if err := json.Unmarshal([]byte(stateJSON), &state); err == nil {
				strat.LoadState(state)
				log.Info().Msg("Strategy state recovered from database")
			}
		}

		// 4. Initialize UI
		m := ui.NewModel(cfg.ActiveExchange, symbol, cfg.DryRun, dbStore, strategyName, cfg.StrategyParameters, cfg.OrderQuantity)
		p := tea.NewProgram(m, tea.WithAltScreen())

		// 5. Background Trading Loop (WebSocket & Execution)
		go func(cmdCtx context.Context) {
			defer func() {
				if r := recover(); r != nil {
					p.Send(ui.AuditMsg(fmt.Sprintf("PANIC: %v", r)))
				}
			}()

			marketDataCh := make(chan exchange.MarketData, 100)

			err := exch.SubscribePrice(cmdCtx, symbol, marketDataCh)
			if err != nil {
				log.Fatal().Err(err).Msg("Failed to subscribe to real-time market data")
			}

			p.Send(ui.AuditMsg("STREAM: Connected to real-time market data"))

			for md := range marketDataCh {
				p.Send(ui.PriceMsg(md))

				signal := strat.OnMarketData(md)
				
				// Only send to UI if it's not WAIT, to avoid spam
				if signal.Type != strategy.Wait {
					p.Send(ui.SignalMsg(signal))
				}

				if signal.Type == strategy.Buy || signal.Type == strategy.Sell {
					side := exchange.Buy
					if signal.Type == strategy.Sell {
						side = exchange.Sell
					}
					
					// Use configured order quantity
					quantity := cfg.OrderQuantity
					totalProfit := signal.Profit * quantity

					if cfg.DryRun {
						p.Send(ui.AuditMsg(fmt.Sprintf("[DRY RUN] Would execute %s order for %.4f at Market (P&L: %.4f)", side, quantity, totalProfit)))
						p.Send(ui.TradeExecutedMsg{})
						
						dbStore.SaveTrade(database.Trade{
							Symbol:    symbol,
							Side:      string(side),
							Price:     md.Price,
							Quantity:  quantity,
							Profit:    totalProfit,
							Timestamp: time.Now(),
							Reason:    fmt.Sprintf("[DRY RUN] %s", signal.Reason),
						})
					} else {
						// Slippage Protection: Use PROTECTED LIMIT order instead of MARKET
						slippageMult := 1.0 + (cfg.SlippagePct / 100.0)
						if side == exchange.Sell {
							slippageMult = 1.0 - (cfg.SlippagePct / 100.0)
						}
						protectedPrice := md.Price * slippageMult

						// Format with precision
						fmtQty := truncateFloat(quantity, symbolInfo.BasePrecision)
						fmtPrice := strconv.FormatFloat(protectedPrice, 'f', symbolInfo.QuotePrecision, 64)
						
						fQty, _ := strconv.ParseFloat(fmtQty, 64)
						fPrice, _ := strconv.ParseFloat(fmtPrice, 64)

						// Validation
						if fQty < symbolInfo.MinQty {
							p.Send(ui.AuditMsg(fmt.Sprintf("SKIP: Quantity %.4f < MinQty %.4f", fQty, symbolInfo.MinQty)))
							continue
						}
						if fQty*fPrice < symbolInfo.MinNotional {
							p.Send(ui.AuditMsg(fmt.Sprintf("SKIP: Notional %.2f < MinNotional %.2f", fQty*fPrice, symbolInfo.MinNotional)))
							continue
						}

						p.Send(ui.AuditMsg(fmt.Sprintf("EXEC: Placing %s %s order (Qty: %s, Limit: %s)", side, exchange.Limit, fmtQty, fmtPrice)))
						
						execCtx, execCancel := context.WithTimeout(context.Background(), 5*time.Second)
						err := exch.ExecuteOrder(execCtx, symbol, side, exchange.Limit, fQty, fPrice)
						execCancel()

						if err != nil {
							p.Send(ui.AuditMsg(fmt.Sprintf("FAIL: %s order error: %v", side, err)))
						} else {
							p.Send(ui.AuditMsg(fmt.Sprintf("SUCCESS: %s order executed (P&L: %.4f)", side, totalProfit)))
							p.Send(ui.TradeExecutedMsg{})

							dbStore.SaveTrade(database.Trade{
								Symbol:    symbol,
								Side:      string(side),
								Price:     fPrice,
								Quantity:  fQty,
								Profit:    totalProfit,
								Timestamp: time.Now(),
								Reason:    signal.Reason,
							})
						}
					}

					// Persist Strategy State
					state := strat.GetState()
					if stateBytes, err := json.Marshal(state); err == nil {
						dbStore.SaveState("strategy_state", string(stateBytes))
					}
				}
			}
		}(ctx)

		// 6. Background Account Polling Loop
		go func(cmdCtx context.Context) {
			ticker := time.NewTicker(2 * time.Second) // Increased speed for better cockpit experience
			defer ticker.Stop()
			
			pollFunc := func() {
				startTime := time.Now()
				fetchCtx, fetchCancel := context.WithTimeout(cmdCtx, 8*time.Second) 
				defer fetchCancel()

				if exchCfg.Key == "" || exchCfg.Secret == "" {
					p.Send(ui.AuditMsg(fmt.Sprintf("WARN: No API keys for %s; skipping account data", cfg.ActiveExchange)))
					return
				}

				// 1. Balances & Orders
				balances, err := exch.GetBalances(fetchCtx)
				if err == nil {
					p.Send(ui.BalanceUpdateMsg(balances))
				}

				orders, err := exch.GetOpenOrders(fetchCtx, symbol)
				if err == nil {
					p.Send(ui.OpenOrdersUpdateMsg(orders))
				}

				// 2. Order Book (Tactical)
				book, err := exch.GetOrderBook(fetchCtx, symbol, 5)
				if err == nil {
					p.Send(ui.OrderBookUpdateMsg(book))
				} else {
					p.Send(ui.AuditMsg(fmt.Sprintf("WARN: Order book update failed: %v", err)))
				}

				// 3. Market Pulse (Context - Try to get BTC/ETH)
				pulse := make(map[string]float64)
				btcSym := "BTCUSDT"
				ethSym := "ETHUSDT"
				if cfg.ActiveExchange == "bitkub" {
					btcSym = "THB_BTC"
					ethSym = "THB_ETH"
				}
				if btcP, err := exch.GetPrice(fetchCtx, btcSym); err == nil { pulse["BTC"] = btcP }
				if ethP, err := exch.GetPrice(fetchCtx, ethSym); err == nil { pulse["ETH"] = ethP }
				p.Send(ui.MarketPulseMsg(pulse))

				// 4. Bot Health (Latency)
				p.Send(ui.LatencyUpdateMsg(time.Since(startTime)))

				stats, err := dbStore.GetStats()
				if err == nil {
					p.Send(ui.StatsUpdateMsg(stats))
				}
			}

			// Initial fetch
			pollFunc()

			for {
				select {
				case <-cmdCtx.Done():
					return
				case <-ticker.C:
					pollFunc()
				}
			}
		}(ctx)

		if _, err := p.Run(); err != nil {
			log.Fatal().Err(err).Msg("Failed to run UI")
		}
	}
}

// truncateFloat formats a float by truncating to n decimal places (no rounding)
func truncateFloat(f float64, precision int) string {
	shift := math.Pow(10, float64(precision))
	truncated := math.Trunc(f*shift) / shift
	return strconv.FormatFloat(truncated, 'f', precision, 64)
}
