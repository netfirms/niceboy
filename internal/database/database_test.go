package database

import (
	"os"
	"testing"
	"time"
)

func TestSQLiteStore_ClearTrades(t *testing.T) {
	dbPath := "test_clear.db"
	defer os.Remove(dbPath)

	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// 1. Add some trades
	trade := Trade{
		Symbol:    "BTCUSDT",
		Side:      "SELL",
		Price:     50000.0,
		Quantity:  1.0,
		Profit:    100.0,
		Timestamp: time.Now(),
		Reason:    "Test",
	}

	if err := store.SaveTrade(trade); err != nil {
		t.Fatalf("failed to save trade: %v", err)
	}

	// 2. Verify trade exists
	stats, err := store.GetStats()
	if err != nil {
		t.Fatalf("failed to get stats: %v", err)
	}
	if stats.TotalTrades != 1 {
		t.Errorf("expected 1 trade, got %d", stats.TotalTrades)
	}

	// 3. Clear trades
	if err := store.ClearTrades(); err != nil {
		t.Fatalf("failed to clear trades: %v", err)
	}

	// 4. Verify trades are gone
	stats, err = store.GetStats()
	if err != nil {
		t.Fatalf("failed to get stats after clear: %v", err)
	}
	if stats.TotalTrades != 0 {
		t.Errorf("expected 0 trades after clear, got %d", stats.TotalTrades)
	}
	if stats.TotalProfit != 0 {
		t.Errorf("expected 0 profit after clear, got %f", stats.TotalProfit)
	}

	recent, err := store.GetRecentTrades(10)
	if err != nil {
		t.Fatalf("failed to get recent trades: %v", err)
	}
	if len(recent) != 0 {
		t.Errorf("expected 0 recent trades, got %d", len(recent))
	}
}

func TestSQLiteStore_ExportTradesToCSV(t *testing.T) {
	dbPath := "test_export.db"
	csvPath := "test_export.csv"
	defer os.Remove(dbPath)
	defer os.Remove(csvPath)

	store, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// 1. Add some trades
	trades := []Trade{
		{Symbol: "BTCUSDT", Side: "BUY", Price: 50000.0, Quantity: 1.0, Profit: 0.0, Timestamp: time.Now(), Reason: "Entry"},
		{Symbol: "BTCUSDT", Side: "SELL", Price: 51000.0, Quantity: 1.0, Profit: 1000.0, Timestamp: time.Now().Add(time.Minute), Reason: "TP"},
	}

	for _, tr := range trades {
		if err := store.SaveTrade(tr); err != nil {
			t.Fatalf("failed to save trade: %v", err)
		}
	}

	// 2. Export to CSV
	if err := store.ExportTradesToCSV(csvPath); err != nil {
		t.Fatalf("export failed: %v", err)
	}

	// 3. Verify file exists
	if _, err := os.Stat(csvPath); os.IsNotExist(err) {
		t.Fatalf("CSV file was not created")
	}

	// 4. Verify content (basic check)
	content, err := os.ReadFile(csvPath)
	if err != nil {
		t.Fatalf("failed to read CSV: %v", err)
	}
	if len(content) == 0 {
		t.Errorf("CSV file is empty")
	}
}
