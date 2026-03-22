package ui

import (
	"fmt"
	"strings"
	"time"

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

	AuditLog []string
	Viewport viewport.Model
	Ready    bool
}

func NewModel(exchangeName, symbol string) Model {
	return Model{
		ExchangeName: exchangeName,
		Symbol:       symbol,
		Status:       "Initializing...",
		Balances:     make(map[string]float64),
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		headerHeight := 10 // Increased to account for the new boxes
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
		m.Status = "Connected"

	case SignalMsg:
		m.Signal = strategy.Signal(msg)
		m.addAudit(fmt.Sprintf("[%s] %s: %s (%s)", 
			time.Now().Format("15:04:05"), 
			m.Signal.Type, 
			m.Signal.Reason,
			fmt.Sprintf("%.2f", m.Signal.Price)))

	case AuditMsg:
		m.addAudit(string(msg))

	case BalanceUpdateMsg:
		m.Balances = msg

	case OpenOrdersUpdateMsg:
		m.OpenOrders = msg

	case TradeExecutedMsg:
		m.TradeCount++

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}

	m.Viewport, cmd = m.Viewport.Update(msg)
	return m, cmd
}

func (m *Model) addAudit(line string) {
	m.AuditLog = append(m.AuditLog, line)
	if len(m.AuditLog) > 100 {
		m.AuditLog = m.AuditLog[1:]
	}
	m.Viewport.SetContent(strings.Join(m.AuditLog, "\n"))
	m.Viewport.GotoBottom()
}

func (m Model) View() string {
	if !m.Ready {
		return "\n  Initializing dashboard..."
	}

	// 1. Header Area
	title := headerStyle.Render(fmt.Sprintf("⚡ niceboy ⚡\n%s : %s", strings.ToUpper(m.ExchangeName), m.Symbol))

	// 2. Stats Box
	statsContent := fmt.Sprintf(
		"Status:  %s\nPrice:   %s\nTrades:  %d\nUpdated: %s",
		m.Status,
		priceStyle.Render(fmt.Sprintf("$%.4f", m.Price)),
		m.TradeCount,
		m.LastPoll.Format("15:04:05.000"),
	)
	statsBox := boxStyle.Render(statsContent)

	// 3. Signal Box
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
		"Current Signal: %s\nStrategy Logic: %s",
		signalView,
		m.Signal.Reason,
	)
	if m.Signal.Reason == "" {
		signalContent = fmt.Sprintf("Current Signal: %s\nStrategy Logic: Collecting Price Data...", signalView)
	}

	signalBox := boxStyle.Render(signalContent)

	// Layout the Header and top boxes
	topSection := lipgloss.JoinHorizontal(lipgloss.Center, title, statsBox, signalBox)

	// 4. Balances and Orders Box
	var balancesStr []string
	for k, v := range m.Balances {
		balancesStr = append(balancesStr, fmt.Sprintf("%s: %.4f", k, v))
	}
	balText := "Balances:\n" + strings.Join(balancesStr, " | ")
	if len(m.Balances) == 0 {
		balText = "Balances: Loading..."
	}

	ordersStr := fmt.Sprintf("Open Orders: %d", len(m.OpenOrders))
	for _, o := range m.OpenOrders {
		os := "B"
		if o.Side == exchange.Sell {
			os = "S"
		}
		ordersStr += fmt.Sprintf("\n[%s] %s %.4f @ %.2f", o.ID, os, o.Quantity, o.Price)
	}
	
	accContent := fmt.Sprintf("%s\n──────────────────\n%s", balText, ordersStr)
	accBox := boxStyle.Render(accContent)

	midSection := lipgloss.JoinHorizontal(lipgloss.Left, accBox)

	// 5. Build final view
	return fmt.Sprintf("%s\n%s\n\n%s\n%s", 
		topSection,
		midSection,
		m.Viewport.View(),
		auditStyle.Render(" [q:quit] "))
}
