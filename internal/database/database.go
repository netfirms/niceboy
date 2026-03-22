package database

import (
	"database/sql"
	"fmt"
	_ "modernc.org/sqlite"
	"time"
)

// Trade represents a persistent record of an executed trade
type Trade struct {
	ID        int64
	Symbol    string
	Side      string // BUY, SELL
	Price     float64
	Quantity  float64
	Profit    float64
	Timestamp time.Time
	Reason    string
}

// TradingStats represents the aggregated performance metrics
type TradingStats struct {
	TotalTrades   int
	TotalProfit   float64
	WinRate       float64
	AverageProfit float64
}

// Store defines the interface for project-wide persistent storage
type Store interface {
	Close() error
	SaveTrade(t Trade) error
	GetRecentTrades(limit int) ([]Trade, error)
	SaveState(key, value string) error
	GetState(key string) (string, error)
	GetStats() (TradingStats, error)
	ClearTrades() error
}

// SQLiteStore is a high-performance local database driver
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore initializes the database and returns a Store implementation
func NewSQLiteStore(path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	// Enable WAL mode for high-performance concurrent access
	if _, err := db.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		return nil, fmt.Errorf("failed to enable WAL: %v", err)
	}

	s := &SQLiteStore{db: db}
	if err := s.initSchema(); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *SQLiteStore) initSchema() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS trades (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			symbol TEXT,
			side TEXT,
			price REAL,
			quantity REAL,
			profit REAL,
			reason TEXT,
			timestamp DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS bot_state (
			key TEXT PRIMARY KEY,
			value TEXT,
			updated_at DATETIME
		)`,
	}

	for _, q := range queries {
		if _, err := s.db.Exec(q); err != nil {
			return fmt.Errorf("schema error: %v", err)
		}
	}
	return nil
}

// Close gracefully shuts down the database connection
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// SaveTrade appends a new trade record to the database
func (s *SQLiteStore) SaveTrade(t Trade) error {
	_, err := s.db.Exec(
		"INSERT INTO trades (symbol, side, price, quantity, profit, reason, timestamp) VALUES (?, ?, ?, ?, ?, ?, ?)",
		t.Symbol, t.Side, t.Price, t.Quantity, t.Profit, t.Reason, t.Timestamp,
	)
	return err
}

// GetRecentTrades retrieves the last N trades from history
func (s *SQLiteStore) GetRecentTrades(limit int) ([]Trade, error) {
	rows, err := s.db.Query("SELECT id, symbol, side, price, quantity, profit, reason, timestamp FROM trades ORDER BY timestamp DESC LIMIT ?", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var trades []Trade
	for rows.Next() {
		var t Trade
		err := rows.Scan(&t.ID, &t.Symbol, &t.Side, &t.Price, &t.Quantity, &t.Profit, &t.Reason, &t.Timestamp)
		if err != nil {
			return nil, err
		}
		trades = append(trades, t)
	}
	return trades, nil
}

// SaveState persists a key-value pair for bot state recovery
func (s *SQLiteStore) SaveState(key, value string) error {
	_, err := s.db.Exec(
		"INSERT OR REPLACE INTO bot_state (key, value, updated_at) VALUES (?, ?, ?)",
		key, value, time.Now(),
	)
	return err
}

// GetState retrieves a persisted value by key
func (s *SQLiteStore) GetState(key string) (string, error) {
	var val string
	err := s.db.QueryRow("SELECT value FROM bot_state WHERE key = ?", key).Scan(&val)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return val, err
}

// GetStats calculates aggregated performance metrics from the trades table
func (s *SQLiteStore) GetStats() (TradingStats, error) {
	var stats TradingStats

	// We consider a "trade" as a completed SELL action
	query := `
		SELECT 
			COUNT(*), 
			IFNULL(SUM(profit), 0), 
			IFNULL(AVG(profit), 0),
			(SELECT COUNT(*) FROM trades WHERE side = 'SELL' AND profit > 0)
		FROM trades 
		WHERE side = 'SELL'
	`

	var count, wins int
	err := s.db.QueryRow(query).Scan(&count, &stats.TotalProfit, &stats.AverageProfit, &wins)
	if err != nil {
		return stats, err
	}

	stats.TotalTrades = count
	if count > 0 {
		stats.WinRate = (float64(wins) / float64(count)) * 100.0
	}

	return stats, nil
}

// ClearTrades removes all trade records from the database
func (s *SQLiteStore) ClearTrades() error {
	_, err := s.db.Exec("DELETE FROM trades")
	return err
}
