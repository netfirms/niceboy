package ui

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"niceboy/internal/database"
	"niceboy/internal/exchange"
	"niceboy/internal/strategy"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Message types
type PriceMsg exchange.MarketData
type SignalMsg strategy.Signal
type AuditMsg string
type TradeExecutedMsg struct{}
type BalanceUpdateMsg map[string]float64
type OpenOrdersUpdateMsg []exchange.Order
type TradesUpdateMsg []database.Trade
type StatsUpdateMsg database.TradingStats

const (
	TabCockpit  = 0
	TabAccount  = 1
	TabLogs     = 2
	TabHistory  = 3
	TabStrategy = 4
)

// Styles
var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#00ffd5")).
			Padding(1, 2)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#333333")).
			Padding(0, 1)

	priceStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ffcc00")).
			Bold(true)

	auditStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).
			Italic(true)

	buyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#00ff00"))

	sellStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#ff0000"))

	waitStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#aaaaaa"))
)

type Model struct {
	ExchangeName string
	Symbol       string
	Price        float64
	Signal       strategy.Signal
	LastPoll     time.Time
	Status       string
	TradeCount   int

	Balances   map[string]float64
	OpenOrders []exchange.Order

	ActiveTab int
	Width     int
	Height    int

	AuditLog []string
	Viewport viewport.Model
	Ready    bool
	DryRun   bool
	Trades   []database.Trade
	DB       database.Store
	Stats    database.TradingStats

	StrategyName   string
	StrategyParams map[string]interface{}
	OrderQuantity  float64

	StatusMsg      string
	StatusExpiry   time.Time

	PriceHistory []float64
	TradeMarkers map[int]string // index -> "B" or "S"

	OrderBook  exchange.OrderBook
	AvgLatency time.Duration
	MarketPulse map[string]float64 // BTC, ETH prices
}

type OrderBookUpdateMsg exchange.OrderBook
type LatencyUpdateMsg time.Duration
type MarketPulseMsg map[string]float64

func NewModel(exchangeName, symbol string, dryRun bool, db database.Store, strategyName string, strategyParams map[string]interface{}, orderQuantity float64) Model {
	return Model{
		ExchangeName:   exchangeName,
		Symbol:         symbol,
		Status:         "Initializing...",
		Balances:       make(map[string]float64),
		ActiveTab:      TabCockpit,
		DryRun:         dryRun,
		DB:             db,
		StrategyName:   strategyName,
		StrategyParams: strategyParams,
		OrderQuantity:  orderQuantity,
		TradeMarkers:   make(map[int]string),
		MarketPulse:    make(map[string]float64),
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		headerHeight := 10 // Adjusted for header + tabs
		footerHeight := 1
		if !m.Ready {
			m.Viewport = viewport.New(msg.Width, msg.Height-headerHeight-footerHeight)
			m.Ready = true
		} else {
			m.Viewport.Width = msg.Width
			m.Viewport.Height = msg.Height - headerHeight - footerHeight
		}

	case PriceMsg:
		m.Price = msg.Price
		m.LastPoll = time.Unix(msg.Time/1000, 0)
		// Update price history (last 5000 points)
		m.PriceHistory = append(m.PriceHistory, m.Price)
		if len(m.PriceHistory) > 5000 {
			m.PriceHistory = m.PriceHistory[1:]
			// Shift markers
			newMarkers := make(map[int]string)
			for idx, mark := range m.TradeMarkers {
				if idx > 0 {
					newMarkers[idx-1] = mark
				}
			}
			m.TradeMarkers = newMarkers
		}

	case SignalMsg:
		m.Signal = strategy.Signal(msg)
		m.addAudit(fmt.Sprintf("[%s] %s: %s (%s)", 
			time.Now().Format("15:04:05"), 
			m.Signal.Type, 
			m.Signal.Reason,
			fmt.Sprintf("%.2f", m.Signal.Price)))
		
		// Record trade marker if signal is BUY or SELL and price is valid
		if m.Signal.Type == strategy.Buy || m.Signal.Type == strategy.Sell {
			marker := "B"
			if m.Signal.Type == strategy.Sell { marker = "S" }
			if len(m.PriceHistory) > 0 {
				m.TradeMarkers[len(m.PriceHistory)-1] = marker
			}
		}

	case AuditMsg:
		m.addAudit(string(msg))

	case BalanceUpdateMsg:
		m.Balances = msg

	case OpenOrdersUpdateMsg:
		m.OpenOrders = msg

	case TradesUpdateMsg:
		m.Trades = msg
		if m.ActiveTab == TabHistory {
			m.updateViewportContent()
		}

	case StatsUpdateMsg:
		m.Stats = database.TradingStats(msg)

	case OrderBookUpdateMsg:
		m.OrderBook = exchange.OrderBook(msg)

	case LatencyUpdateMsg:
		m.AvgLatency = time.Duration(msg)

	case MarketPulseMsg:
		for k, v := range msg {
			m.MarketPulse[k] = v
		}

	case TradeExecutedMsg:
		m.TradeCount++
		// Always refresh history to ensure Cockpit mini-history is up to date
		return m, m.fetchTradesCmd()

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab", "right":
			m.ActiveTab = (m.ActiveTab + 1) % 5
			if m.ActiveTab == TabHistory {
				return m, m.fetchTradesCmd()
			}
			if m.ActiveTab == TabLogs || m.ActiveTab == TabHistory || m.ActiveTab == TabStrategy {
				m.updateViewportContent()
			}
		case "left":
			m.ActiveTab = (m.ActiveTab - 1 + 5) % 5
			if m.ActiveTab == TabHistory {
				return m, m.fetchTradesCmd()
			}
			if m.ActiveTab == TabLogs || m.ActiveTab == TabHistory || m.ActiveTab == TabStrategy {
				m.updateViewportContent()
			}
		case "x":
			// Reset local state immediately for snappy UI
			m.Trades = []database.Trade{}
			m.Stats = database.TradingStats{}
			m.TradeCount = 0
			m.AuditLog = []string{}
			m.Viewport.SetContent("")
			return m, tea.Batch(
				m.clearTradesCmd(), 
				m.fetchStatsCmd(), 
				m.fetchTradesCmd(),
				func() tea.Msg { return AuditMsg("[SYSTEM] All performance and audit records cleared.") },
			)
		case "e":
			m.StatusMsg = "✅ Data Exported"
			m.StatusExpiry = time.Now().Add(3 * time.Second)
			return m, tea.Batch(m.exportCmd(), m.exportAuditCmd())
		case "b":
			m.StatusMsg = "⚡ FORCING BUY..."
			m.StatusExpiry = time.Now().Add(2 * time.Second)
			m.addAudit("[TACTICAL] User triggered manual BUY")
			// In a real implementation, we'd trigger a buy command here
		case "s":
			m.StatusMsg = "⚡ FORCING SELL..."
			m.StatusExpiry = time.Now().Add(2 * time.Second)
			m.addAudit("[TACTICAL] User triggered manual SELL")
		case "k":
			m.StatusMsg = "🛑 KILL SWITCH ACTIVATED"
			m.addAudit("[EMERGENCY] KILL SWITCH TRIGGERED BY USER")
			return m, tea.Quit // For now, just exit. Engines may need deeper logic.
		}
	}

	// Clear status message after expiry
	if m.StatusMsg != "" && time.Now().After(m.StatusExpiry) {
		m.StatusMsg = ""
	}

	if m.ActiveTab == TabLogs {
		m.Viewport, cmd = m.Viewport.Update(msg)
	}
	return m, cmd
}

func (m Model) fetchTradesCmd() tea.Cmd {
	return func() tea.Msg {
		if m.DB == nil {
			return TradesUpdateMsg{}
		}
		trades, err := m.DB.GetRecentTrades(50)
		if err != nil {
			return TradesUpdateMsg{}
		}
		return TradesUpdateMsg(trades)
	}
}

func (m Model) fetchStatsCmd() tea.Cmd {
	return func() tea.Msg {
		if m.DB == nil {
			return StatsUpdateMsg{}
		}
		stats, err := m.DB.GetStats()
		if err != nil {
			return StatsUpdateMsg{}
		}
		return StatsUpdateMsg(stats)
	}
}

func (m Model) clearTradesCmd() tea.Cmd {
	return func() tea.Msg {
		if m.DB == nil {
			return AuditMsg("[ERROR] No database connection")
		}
		if err := m.DB.ClearTrades(); err != nil {
			return AuditMsg(fmt.Sprintf("[ERROR] Clear failed: %v", err))
		}
		return AuditMsg("[SYSTEM] Performance records cleared.")
	}
}

func (m Model) exportCmd() tea.Cmd {
	return func() tea.Msg {
		if m.DB == nil {
			return AuditMsg("[ERROR] Cannot export: No database")
		}
		filename := fmt.Sprintf("trades_export_%s.csv", time.Now().Format("20060102_150405"))
		if err := m.DB.ExportTradesToCSV(filename); err != nil {
			return AuditMsg(fmt.Sprintf("[ERROR] CSV Export failed: %v", err))
		}
		return AuditMsg(fmt.Sprintf("[SYSTEM] Trade history exported to %s", filename))
	}
}

func (m Model) exportAuditCmd() tea.Cmd {
	return func() tea.Msg {
		filename := fmt.Sprintf("audit_log_%s.txt", time.Now().Format("20060102_150405"))
		content := strings.Join(m.AuditLog, "\n")
		if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
			return AuditMsg(fmt.Sprintf("[ERROR] Audit Export failed: %v", err))
		}
		return AuditMsg(fmt.Sprintf("[SYSTEM] Audit log exported to %s", filename))
	}
}

func (m *Model) addAudit(line string) {
	m.AuditLog = append(m.AuditLog, line)
	if len(m.AuditLog) > 100 {
		m.AuditLog = m.AuditLog[1:]
	}
	if m.ActiveTab == TabLogs {
		m.Viewport.SetContent(strings.Join(m.AuditLog, "\n"))
		m.Viewport.GotoBottom()
	}
}

func (m Model) View() string {
	if !m.Ready {
		return "\n  Initializing dashboard..."
	}

	// 1. Header Area
	titleText := fmt.Sprintf("⚡ niceboy ⚡\n%s : %s", strings.ToUpper(m.ExchangeName), m.Symbol)
	if m.DryRun {
		titleText += " [DRY RUN]"
	}
	title := headerStyle.Render(titleText)

	// 2. Tabs
	activeTabStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#00ffd5")).Padding(0, 2).Bold(true).Foreground(lipgloss.Color("#ffffff"))
	inactiveTabStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#555555")).Padding(0, 2).Foreground(lipgloss.Color("#888888"))

	var tabCockpit, tabAccount, tabLogs, tabHist, tabStrat string
	tabCockpit = inactiveTabStyle.Render("Cockpit")
	tabAccount = inactiveTabStyle.Render("Account")
	tabLogs = inactiveTabStyle.Render("Audit Logs")
	tabHist = inactiveTabStyle.Render("History")
	tabStrat = inactiveTabStyle.Render("Strategy")

	switch m.ActiveTab {
	case TabCockpit:
		tabCockpit = activeTabStyle.Render("Cockpit")
	case TabAccount:
		tabAccount = activeTabStyle.Render("Account")
	case TabLogs:
		tabLogs = activeTabStyle.Render("Audit Logs")
	case TabHistory:
		tabHist = activeTabStyle.Render("History")
	case TabStrategy:
		tabStrat = activeTabStyle.Render("Strategy")
	}
	tabsRow := lipgloss.JoinHorizontal(lipgloss.Top, tabCockpit, tabAccount, tabLogs, tabHist, tabStrat)
	
	headerSection := lipgloss.JoinVertical(lipgloss.Center, title, tabsRow)

	if m.ActiveTab == TabLogs || m.ActiveTab == TabHistory || m.ActiveTab == TabStrategy {
		return fmt.Sprintf("%s\n\n%s\n%s",
			lipgloss.PlaceHorizontal(m.Width, lipgloss.Center, headerSection),
			m.Viewport.View(),
			lipgloss.PlaceHorizontal(m.Width, lipgloss.Center, auditStyle.Render(" [tab:switch view] [e:export] [x:clear] [q:quit] ")))
	}

	if m.ActiveTab == TabCockpit {
		return m.renderCockpit(headerSection)
	}

	// --- Dashboard View Building (Renamed to Account) ---
	
	// 3. Stats Box
	statsContent := fmt.Sprintf(
		"Status:  %s\nPrice:   %s\nTrades:  %d\nUpdated: %s",
		m.Status,
		priceStyle.Render(fmt.Sprintf("$%.4f", m.Price)),
		m.TradeCount,
		m.LastPoll.Format("15:04:05.000"),
	)
	statsBox := boxStyle.Width(35).Height(5).Render(statsContent)

	// 4. Signal Box
	signalStr := m.Signal.Type.String()
	var signalView string
	switch m.Signal.Type {
	case strategy.Buy:
		signalView = buyStyle.Render(signalStr)
	case strategy.Sell:
		signalView = sellStyle.Render(signalStr)
	default:
		signalView = waitStyle.Render(signalStr)
	}
	
	signalContent := fmt.Sprintf(
		"Current Signal: %s\nStrategy Logic:\n%s",
		signalView,
		m.Signal.Reason,
	)
	if m.Signal.Reason == "" {
		signalContent = fmt.Sprintf("Current Signal: %s\nStrategy Logic:\nCollecting Price Data...", signalView)
	}

	signalBox := boxStyle.Width(35).Height(5).Render(signalContent)

	// 5. Balances Box (Updated with USDT/USDC priority)
	var balancesStr []string
	keys := make([]string, 0, len(m.Balances))
	for k := range m.Balances {
		keys = append(keys, k)
	}

	sort.Slice(keys, func(i, j int) bool {
		ki := keys[i]
		kj := keys[j]
		if ki == "USDT" { return true }
		if kj == "USDT" { return false }
		if ki == "USDC" { return true }
		if kj == "USDC" { return false }
		return ki < kj
	})

	for _, k := range keys {
		v := m.Balances[k]
		balancesStr = append(balancesStr, fmt.Sprintf("%s: %.4f", k, v))
	}
	balText := strings.Join(balancesStr, "\n")
	if len(m.Balances) == 0 {
		balText = "Loading..."
	}
	accBox := boxStyle.Width(35).Render("Balances:\n─────────\n" + balText)

	// 6. Orders Box
	ordersStr := ""
	for _, o := range m.OpenOrders {
		os := "B"
		if o.Side == exchange.Sell {
			os = "S"
		}
		ordersStr += fmt.Sprintf("[%s] %s %.4f @ %.2f\n", o.ID, os, o.Quantity, o.Price)
	}
	if len(m.OpenOrders) == 0 {
		ordersStr = "No active orders."
	}
	
	ordersBox := boxStyle.Width(35).Render(fmt.Sprintf("Open Orders: %d\n──────────────\n%s", len(m.OpenOrders), strings.TrimSuffix(ordersStr, "\n")))

	// 7. Performance Box
	perfContent := fmt.Sprintf(
		"Total P&L:  %s\nWin Rate:   %.1f%%\nAvg Profit: %.4f\nTotal Exec: %d",
		priceStyle.Render(fmt.Sprintf("$%.4f", m.Stats.TotalProfit)),
		m.Stats.WinRate,
		m.Stats.AverageProfit,
		m.Stats.TotalTrades,
	)
	perfBox := boxStyle.Width(35).Render("Performance:\n────────────\n" + perfContent)

	// Layout the blocks
	topWidgets := lipgloss.JoinHorizontal(lipgloss.Top, statsBox, signalBox, perfBox)
	midWidgets := lipgloss.JoinHorizontal(lipgloss.Top, accBox, ordersBox)
	
	dashboardMatrix := lipgloss.JoinVertical(lipgloss.Center, topWidgets, midWidgets)

	statusLine := auditStyle.Render(" [tab:switch view] [e:export] [x:clear] [q:quit] ")
	if m.StatusMsg != "" {
		statusLine = lipgloss.NewStyle().Foreground(lipgloss.Color("#00ffd5")).Bold(true).Render(" " + m.StatusMsg + " ")
	}

	return fmt.Sprintf("%s\n\n%s\n\n%s", 
		lipgloss.PlaceHorizontal(m.Width, lipgloss.Center, headerSection),
		lipgloss.PlaceHorizontal(m.Width, lipgloss.Center, dashboardMatrix),
		lipgloss.PlaceHorizontal(m.Width, lipgloss.Center, statusLine))
}

func (m Model) renderCockpit(headerSection string) string {
	// 1. Position Tracker
	pnl := 0.0
	pnlPct := 0.0
	inPosition := m.Signal.EntryPrice > 0 && m.Price > 0
	
	posTitle := "POSITION: NONE"
	posColor := lipgloss.Color("#555555")
	if inPosition {
		pnl = m.Price - m.Signal.EntryPrice
		pnlPct = (pnl / m.Signal.EntryPrice) * 100.0
		posTitle = fmt.Sprintf("POSITION: LONG %s", m.Symbol)
		if pnl >= 0 {
			posColor = lipgloss.Color("#00ff00")
		} else {
			posColor = lipgloss.Color("#ff0000")
		}
	}

	pnlText := "N/A"
	if inPosition {
		pnlText = fmt.Sprintf("%+.2f (%+.2f%%)", pnl, pnlPct)
	}

	posBox := boxStyle.Width(35).BorderForeground(posColor).Render(
		fmt.Sprintf("%s\nEntry: $%.2f\nP/L:   %s", 
			lipgloss.NewStyle().Foreground(posColor).Bold(true).Render(posTitle),
			m.Signal.EntryPrice,
			lipgloss.NewStyle().Foreground(posColor).Render(pnlText),
		),
	)

	// 2. Guardrails (SL/TP/Trailing)
	slText := "DISABLED"
	if m.Signal.StopLoss > 0 { slText = fmt.Sprintf("$%.2f", m.Signal.StopLoss) }
	tpText := "DISABLED"
	if m.Signal.TakeProfit > 0 { tpText = fmt.Sprintf("$%.2f", m.Signal.TakeProfit) }
	tsText := "DISABLED"
	if m.Signal.TrailingStop > 0 { tsText = fmt.Sprintf("$%.2f", m.Signal.TrailingStop) }

	guardBox := boxStyle.Width(35).Render(
		fmt.Sprintf("GUARDRAILS:\nStop Loss:  %s\nTake Profit: %s\nTrailing:    %s",
			lipgloss.NewStyle().Foreground(lipgloss.Color("#ff5555")).Render(slText),
			lipgloss.NewStyle().Foreground(lipgloss.Color("#55ff55")).Render(tpText),
			lipgloss.NewStyle().Foreground(lipgloss.Color("#55aaff")).Render(tsText),
		),
	)

	// 3. Strategy Health
	trendStatus := "Neutral"
	trendColor := lipgloss.Color("#888888")
	if strings.Contains(m.Signal.Reason, "Bearish Trend") {
		trendStatus = "Bearish (Suppressed)"
		trendColor = lipgloss.Color("#ff5555")
	} else if strings.Contains(m.Signal.Reason, "Trend Confirmed") {
		trendStatus = "Bullish"
		trendColor = lipgloss.Color("#55ff55")
	}

	healthBox := boxStyle.Width(35).Render(
		fmt.Sprintf("STRATEGY HEALTH:\nTrend:  %s\nStatus: %s",
			lipgloss.NewStyle().Foreground(trendColor).Render(trendStatus),
			m.Status,
		),
	)

	// 4. Large Price & Signal
	priceArea := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#ffcc00")).
		Padding(1, 4).
		Border(lipgloss.DoubleBorder()).
		Render(fmt.Sprintf("$ %.2f", m.Price))
	
	sigView := waitStyle.Render("WAITING")
	if m.Signal.Type == strategy.Buy { sigView = buyStyle.Render("BUY") }
	if m.Signal.Type == strategy.Sell { sigView = sellStyle.Render("SELL") }
	
	signalArea := lipgloss.NewStyle().Padding(1, 4).Render("SIGNAL: " + sigView)

	centerArea := lipgloss.JoinVertical(lipgloss.Center, priceArea, signalArea)

	// 5. Mini History (Last 10)
	histLines := []string{"LAST EXECUTIONS:"}
	limit := 10
	if len(m.Trades) < limit { limit = len(m.Trades) }
	for i := 0; i < limit; i++ {
		t := m.Trades[i]
		side := buyStyle.Render("▲ B")
		if strings.ToUpper(t.Side) == "SELL" {
			side = sellStyle.Render("▼ S")
		}
		histLines = append(histLines, fmt.Sprintf(" • %s: %s %.2f", t.Timestamp.Format("15:04:05"), side, t.Price))
	}
	if len(m.Trades) == 0 { histLines = append(histLines, " • No trades yet") }
	histBox := boxStyle.Width(35).Render(strings.Join(histLines, "\n"))

	// Layout
	topRow := lipgloss.JoinHorizontal(lipgloss.Top, posBox, guardBox, healthBox)
	chartArea := m.renderChart()
	
	depthBox := m.renderOrderBook()
	pulseBox := m.renderMarketPulse()
	
	midRow := lipgloss.JoinHorizontal(lipgloss.Center, centerArea, chartArea, histBox)
	bottomRow := lipgloss.JoinHorizontal(lipgloss.Top, depthBox, pulseBox)
	
	cockpitMatrix := lipgloss.JoinVertical(lipgloss.Center, topRow, midRow, bottomRow)
	
	statusLine := auditStyle.Render(" [tab:switch view] [b:buy] [s:sell] [k:kill] [e:export] [q:quit] ")
	if m.StatusMsg != "" {
		statusLine = lipgloss.NewStyle().Foreground(lipgloss.Color("#00ffd5")).Bold(true).Render(" " + m.StatusMsg + " ")
	}

	return fmt.Sprintf("%s\n\n%s\n\n%s", 
		lipgloss.PlaceHorizontal(m.Width, lipgloss.Center, headerSection),
		lipgloss.PlaceHorizontal(m.Width, lipgloss.Center, cockpitMatrix),
		lipgloss.PlaceHorizontal(m.Width, lipgloss.Center, statusLine))
}

func (m Model) renderOrderBook() string {
	var lines []string
	lines = append(lines, "ORDER BOOK (Top 5 depth)")
	lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("#ff5555")).Render("  PRICE       QUANTITY (ASK)"))
	
	// Asks (Reverse order for visual: Highest Ask on top)
	limit := 5
	if len(m.OrderBook.Asks) < limit { limit = len(m.OrderBook.Asks) }
	for i := limit - 1; i >= 0; i-- {
		a := m.OrderBook.Asks[i]
		lines = append(lines, fmt.Sprintf("  %-10.2f  %-10.4f", a.Price, a.Quantity))
	}
	
	lines = append(lines, "  ─────────── SPREAD ──────────")
	
	// Bids
	limit = 5
	if len(m.OrderBook.Bids) < limit { limit = len(m.OrderBook.Bids) }
	for i := 0; i < limit; i++ {
		b := m.OrderBook.Bids[i]
		lines = append(lines, fmt.Sprintf("  %-10.2f  %-10.4f", b.Price, b.Quantity))
	}
	lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("#55ff55")).Render("  PRICE       QUANTITY (BID)"))

	return boxStyle.Width(35).Height(15).Render(strings.Join(lines, "\n"))
}

func (m Model) renderMarketPulse() string {
	var lines []string
	lines = append(lines, "MARKET PULSE (Context)")
	
	btc := m.MarketPulse["BTC"]
	eth := m.MarketPulse["ETH"]
	
	lines = append(lines, fmt.Sprintf("BTC:  $%.2f", btc))
	lines = append(lines, fmt.Sprintf("ETH:  $%.2f", eth))
	lines = append(lines, "")
	lines = append(lines, "BOT HEALTH:")
	
	latencyColor := lipgloss.Color("#00ff00")
	if m.AvgLatency > 200*time.Millisecond { latencyColor = lipgloss.Color("#ffcc00") }
	if m.AvgLatency > 500*time.Millisecond { latencyColor = lipgloss.Color("#ff5555") }
	
	lines = append(lines, fmt.Sprintf("Latency: %s", 
		lipgloss.NewStyle().Foreground(latencyColor).Render(m.AvgLatency.String())))
	lines = append(lines, fmt.Sprintf("Status:  %s", m.Status))

	return boxStyle.Width(35).Height(15).Render(strings.Join(lines, "\n"))
}

func (m *Model) updateViewportContent() {
	var content string
	switch m.ActiveTab {
	case TabLogs:
		content = strings.Join(m.AuditLog, "\n")
	case TabHistory:
		var histLines []string
		histLines = append(histLines, lipgloss.NewStyle().Bold(true).Render("  TIME     SIDE  PRICE      QTY    REASON"))
		histLines = append(histLines, "  ────────────────────────────────────────────────────────")
		for _, t := range m.Trades {
			side := buyStyle.Render("BUY ")
			if strings.ToUpper(t.Side) == "SELL" { side = sellStyle.Render("SELL") }
			line := fmt.Sprintf("  %s  %s  %-10.2f  %-5.4f  %s", 
				t.Timestamp.Format("15:04:05"), side, t.Price, t.Quantity, t.Reason)
			histLines = append(histLines, line)
		}
		if len(m.Trades) == 0 {
			histLines = append(histLines, "\n  No trade history found.")
		}
		content = strings.Join(histLines, "\n")
	case TabStrategy:
		var stratLines []string
		stratLines = append(stratLines, lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#00ffd5")).Render("  Active Strategy: "+m.StrategyName))
		stratLines = append(stratLines, "  ────────────────────────────────────────────────────────")
		stratLines = append(stratLines, fmt.Sprintf("  Order Size: %.4f", m.OrderQuantity))
		stratLines = append(stratLines, "  Parameters:")
		var keys []string
		for k := range m.StrategyParams { keys = append(keys, k) }
		sort.Strings(keys)
		for _, k := range keys {
			stratLines = append(stratLines, fmt.Sprintf("    • %-15s : %v", k, m.StrategyParams[k]))
		}
		content = strings.Join(stratLines, "\n")
	}
	m.Viewport.SetContent(content)
}

func (m Model) renderChart() string {
	// Dynamically calculate width based on terminal size, with bounds
	width := m.Width - 80 
	if width < 50 { width = 50 }
	if width > 120 { width = 120 } // Cap visual width to prevent extreme stretching
	
	height := 8
	if len(m.PriceHistory) < 2 {
		return boxStyle.Width(width + 2).Height(height).Render("Waiting for data...")
	}

	// 1. Find Min/Max
	minP := m.PriceHistory[0]
	maxP := m.PriceHistory[0]
	for _, p := range m.PriceHistory {
		if p < minP { minP = p }
		if p > maxP { maxP = p }
	}
	if maxP == minP {
		maxP += 0.01
		minP -= 0.01
	}

	// 2. Create Grid (empty)
	grid := make([][]string, height)
	for i := range grid {
		grid[i] = make([]string, width)
		for j := range grid[i] {
			grid[i][j] = " "
		}
	}

	// 3. Plot Points (with downsampling)
	historyLen := len(m.PriceHistory)
	stride := 1
	if historyLen > width {
		stride = historyLen / width
	}

	for i := 0; i < width; i++ {
		// Calculate index in history for this column
		histIdx := i * stride
		if stride > 1 {
			// When downsampling, we take the last point in the bucket to represent the "current" state
			histIdx = (i + 1) * stride - 1
		}
		
		if histIdx >= historyLen {
			if i == 0 { continue }
			break 
		}

		p := m.PriceHistory[histIdx]
		// Normalize P to [0, height-1]
		row := int(((p - minP) / (maxP - minP)) * float64(height-1))
		rowIdx := (height - 1) - row
		
		char := "·"
		if i > 0 {
			prevIdx := (i - 1) * stride
			if stride > 1 { prevIdx = i * stride - 1 }
			prevP := m.PriceHistory[prevIdx]
			if p > prevP { char = "◜" }
			if p < prevP { char = "◟" }
		}
		
		// If ANY marker exists in this bucket (stride range), show it
		bucketStart := i * stride
		bucketEnd := (i + 1) * stride
		for j := bucketStart; j < bucketEnd && j < historyLen; j++ {
			if mark, ok := m.TradeMarkers[j]; ok {
				if mark == "B" {
					char = lipgloss.NewStyle().Foreground(lipgloss.Color("#00ff00")).Bold(true).Render("B")
				} else {
					char = lipgloss.NewStyle().Foreground(lipgloss.Color("#ff0000")).Bold(true).Render("S")
				}
				break // Found a marker in this bucket, stop looking
			}
		}
		
		if rowIdx >= 0 && rowIdx < height {
			grid[rowIdx][i] = char
		}
	}

	// 4. Build output string
	var b strings.Builder
	b.WriteString(fmt.Sprintf("PRICE CHART (Last %d ticks)  Max: %.2f  Min: %.2f\n", historyLen, maxP, minP))
	for r := 0; r < height; r++ {
		for c := 0; c < width; c++ {
			b.WriteString(grid[r][c])
		}
		b.WriteString("\n")
	}

	return boxStyle.Width(width + 4).Height(height + 2).Render(b.String())
}
