package ui

import (
	"fmt"
	"niceboy/internal/exchange"
	"niceboy/internal/strategy"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/viewport"
)

func TestNewModel(t *testing.T) {
	m := NewModel("binance", "BTCUSDT", true)
	if m.ExchangeName != "binance" {
		t.Errorf("expected binance, got %s", m.ExchangeName)
	}
	if !m.DryRun {
		t.Error("expected dry run to be true")
	}
	if m.ActiveTab != TabDashboard {
		t.Error("expected dashboard tab to be active")
	}
}

func TestModelUpdates(t *testing.T) {
	m := NewModel("bitkub", "THB_BTC", false)
	m.Viewport = viewport.New(100, 20)
	
	// Test Price Update
	newModel, _ := m.Update(PriceMsg(exchange.MarketData{Price: 50000.0}))
	m = newModel.(Model)
	if m.Price != 50000.0 {
		t.Errorf("expected price 50000, got %f", m.Price)
	}

	// Test Signal Update
	newModel, _ = m.Update(SignalMsg(strategy.Signal{Type: strategy.Buy, Reason: "Buy now"}))
	m = newModel.(Model)
	if m.Signal.Type != strategy.Buy {
		t.Error("expected buy signal")
	}

	// Test Balance Update
	newModel, _ = m.Update(BalanceUpdateMsg(map[string]float64{"USDT": 100.0}))
	m = newModel.(Model)
	if m.Balances["USDT"] != 100.0 {
		t.Error("expected USDT balance 100")
	}

	// Test Tab Switch
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = newModel.(Model)
	if m.ActiveTab != TabLogs {
		t.Error("expected logs tab")
	}
}

func TestViewRendering(t *testing.T) {
	m := NewModel("binance", "BTCUSDT", true)
	m.Width = 100
	m.Height = 40
	m.Ready = true
	m.Viewport = viewport.New(100, 20)

	// Just ensure View() doesn't panic and returns content
	view := m.View()
	if view == "" {
		t.Error("View returned empty string")
	}
	
	// Look for specific elements in the view
	fmt.Print(view) // for visual debug in test output if needed
}
