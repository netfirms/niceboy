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

// Styles
var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#00ffd5")).
			Background(lipgloss.Color("#1a1a1a")).
			Padding(0, 1).
			MarginBottom(1)

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
	Symbol   string
	Price    float64
	Signal   strategy.Signal
	LastPoll time.Time
	Status   string

	AuditLog []string
	Viewport viewport.Model
	Ready    bool
}

func NewModel(symbol string) Model {
	return Model{
		Symbol: symbol,
		Status: "Initializing...",
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		headerHeight := 5
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

	// Header
	header := headerStyle.Render(fmt.Sprintf(" ⚡ niceboy | %s ", m.Symbol))
	info := fmt.Sprintf(" Status: %s | Last Price: %s | Time: %s ", 
		m.Status, 
		priceStyle.Render(fmt.Sprintf("%.2f", m.Price)),
		m.LastPoll.Format("15:04:05"))

	// Signal Box
	signalLabel := " [ SIGNAL: "
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
	signalBox := fmt.Sprintf("%s%s ] %s", signalLabel, signalView, m.Signal.Reason)

	// Build final view
	return fmt.Sprintf("%s\n%s\n\n%s\n\n%s\n%s", 
		header, 
		info, 
		signalBox,
		m.Viewport.View(),
		auditStyle.Render(" [q:quit] "))
}
