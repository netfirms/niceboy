package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"niceboy/internal/config"
	"niceboy/internal/exchange"
	"niceboy/internal/strategy"
	"niceboy/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	// Parse CLI flags for multi-instance support
	configPath := flag.String("config", "config.yaml", "Path to configuration file")
	logPath := flag.String("log", "niceboy.log", "Path to log file")
	flag.Parse()

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
		log.Fatal().Err(err).Str("path", *configPath).Msg("Failed to load config")
	}

	log.Info().Str("exchange", cfg.ActiveExchange).Msg("Configuration loaded")

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

		// 3. Initialize Strategy
		strategyName := cfg.Strategy
		if strategyName == "" {
			strategyName = "sma_crossover" // Fallback
		}
		
		strat, err := strategy.New(strategyName, cfg.StrategyParameters)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to initialize strategy")
		}

		// 4. Initialize UI
		m := ui.NewModel(cfg.ActiveExchange, symbol, cfg.DryRun)
		p := tea.NewProgram(m, tea.WithAltScreen())

		// 5. Background Trading Loop (WebSocket & Execution)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					p.Send(ui.AuditMsg(fmt.Sprintf("PANIC: %v", r)))
				}
			}()

			marketDataCh := make(chan exchange.MarketData, 100)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			err := exch.SubscribePrice(ctx, symbol, marketDataCh)
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
					
					// Assuming a fixed trade amount for now
					quantity := 0.01

					if cfg.DryRun {
						p.Send(ui.AuditMsg(fmt.Sprintf("[DRY RUN] Would execute %s order for %.4f at Market", side, quantity)))
						p.Send(ui.TradeExecutedMsg{})
					} else {
						p.Send(ui.AuditMsg(fmt.Sprintf("EXEC: Placing %s order for %.4f at Market", side, quantity)))
						
						execCtx, execCancel := context.WithTimeout(context.Background(), 5*time.Second)
						err := exch.ExecuteOrder(execCtx, symbol, side, exchange.Market, quantity, 0)
						execCancel()

						if err != nil {
							p.Send(ui.AuditMsg(fmt.Sprintf("FAIL: %s order error: %v", side, err)))
						} else {
							p.Send(ui.AuditMsg(fmt.Sprintf("SUCCESS: %s order executed", side)))
							p.Send(ui.TradeExecutedMsg{})
						}
					}
				}
			}
		}()

		// 6. Background Account Polling Loop
		go func() {
			ticker := time.NewTicker(5 * time.Second)
			defer ticker.Stop()
			
			// Initial fetch
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			if balances, err := exch.GetBalances(ctx); err == nil {
				p.Send(ui.BalanceUpdateMsg(balances))
			}
			if orders, err := exch.GetOpenOrders(ctx, symbol); err == nil {
				p.Send(ui.OpenOrdersUpdateMsg(orders))
			}
			cancel()

			for range ticker.C {
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				if balances, err := exch.GetBalances(ctx); err == nil {
					p.Send(ui.BalanceUpdateMsg(balances))
				}
				if orders, err := exch.GetOpenOrders(ctx, symbol); err == nil {
					p.Send(ui.OpenOrdersUpdateMsg(orders))
				}
				cancel()
			}
		}()

		if _, err := p.Run(); err != nil {
			log.Fatal().Err(err).Msg("Failed to run UI")
		}
	}
}
