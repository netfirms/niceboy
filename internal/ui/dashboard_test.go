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
	m := NewModel("binance", "BTCUSDT", true, nil, "sma_crossover", map[string]interface{}{"short": 5}, 0.01, "dev", "test")
	if m.ExchangeName != "binance" {
		t.Errorf("expected binance, got %s", m.ExchangeName)
	}
	if !m.DryRun {
		t.Error("expected dry run to be true")
	}
	if m.ActiveTab != TabCockpit {
		t.Error("expected cockpit tab to be active")
	}
	if m.AppVersion != "dev" {
		t.Errorf("expected version dev, got %s", m.AppVersion)
	}
	if m.AppCommit != "test" {
		t.Errorf("expected commit test, got %s", m.AppCommit)
	}
}

func TestModelUpdates(t *testing.T) {
	m := NewModel("bitkub", "THB_BTC", false, nil, "sma_crossover", nil, 0.01, "dev", "test")
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
	if m.ActiveTab != TabAccount {
		t.Errorf("expected account tab (1), got %d", m.ActiveTab)
	}
}

func TestViewRendering(t *testing.T) {
	m := NewModel("binance", "BTCUSDT", false, nil, "sma_crossover", map[string]interface{}{"fast_period": 10.0, "slow_period": 21.0}, 0.001, "dev", "testcommit123")
	m.Width = 100
	m.Height = 40
	m.Ready = true
	m.Viewport = viewport.New(100, 20)

	// Test Sorted Balances (must be on Account Tab)
	m.ActiveTab = TabAccount
	m.Balances = map[string]float64{
		"USDT": 100.0,
		"BTC":  0.5,
		"ETH":  2.0,
	}
	view := m.View()
	// Check if they appear in prioritized order: USDT, BTC, ETH
	usdtIdx := strings.Index(view, "USDT:")
	btcIdx := strings.Index(view, "BTC:")
	ethIdx := strings.Index(view, "ETH:")
	
	if usdtIdx == -1 || btcIdx == -1 || ethIdx == -1 {
		t.Error("One or more balances missing from View")
	}
	
	if !(usdtIdx < btcIdx && btcIdx < ethIdx) {
		t.Errorf("Balances not prioritized correctly in View. Indices: USDT=%d, BTC=%d, ETH=%d", usdtIdx, btcIdx, ethIdx)
	}

	// Check if version string is rendered correctly
	if !strings.Contains(view, "dev-testcom") {
		t.Errorf("expected view to contain 'dev-testcom' version string, but it was missing")
	}
}
