package ui

import (
	"niceboy/internal/exchange"
	"niceboy/internal/strategy"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/viewport"
)

func TestNewModel(t *testing.T) {
	m := NewModel("binance", "BTCUSDT", true, nil, "sma_crossover", map[string]interface{}{"short": 5}, 0.01)
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
	m := NewModel("bitkub", "THB_BTC", false, nil, "sma_crossover", nil, 0.01)
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
	m := NewModel("binance", "BTCUSDT", true, nil, "sma_crossover", map[string]interface{}{"short": 10}, 0.01)
	m.Width = 100
	m.Height = 40
	m.Ready = true
	m.Viewport = viewport.New(100, 20)

	// Test Sorted Balances
	m.Balances = map[string]float64{
		"USDT": 100.0,
		"BTC":  0.5,
		"ETH":  2.0,
	}
	view := m.View()
	
	// Check if they appear in sorted order: BTC, ETH, USDT
	btcIdx := strings.Index(view, "BTC:")
	ethIdx := strings.Index(view, "ETH:")
	usdtIdx := strings.Index(view, "USDT:")
	
	if btcIdx == -1 || ethIdx == -1 || usdtIdx == -1 {
		t.Error("One or more balances missing from View")
	}
	
	if !(btcIdx < ethIdx && ethIdx < usdtIdx) {
		t.Errorf("Balances not sorted correctly in View. Indices: BTC=%d, ETH=%d, USDT=%d", btcIdx, ethIdx, usdtIdx)
	}
}
