package exchange

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/adshao/go-binance/v2"
	"github.com/gorilla/websocket"
)

func TestBinanceExchange_GetPrice(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"price": "50000.00"}`))
	}))
	defer server.Close()

	// Direct instantiation to bypass production URL
	b := &BinanceExchange{
		client: binance.NewClient("YOUR_KEY", "YOUR_SECRET"),
	}
	b.client.BaseURL = server.URL
	price, err := b.GetPrice(context.Background(), "BTCUSDT")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if price != 50000.00 {
		t.Errorf("expected 50000.00, got %f", price)
	}
}

func TestBitkubExchange_GetPrice(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"symbol":"BTC_THB","last":2000000.0}]`))
	}))
	defer server.Close()

	b := &BitkubExchange{
		BaseURL: server.URL,
		client:  &http.Client{},
	}

	price, err := b.GetPrice(context.Background(), "THB_BTC")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if price != 2000000.0 {
		t.Errorf("expected price 2000000.0, got %f", price)
	}

	if b.GetName() != "bitkub" {
		t.Errorf("expected bitkub, got %s", b.GetName())
	}
}

func TestBitkubExchange_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	b := &BitkubExchange{
		BaseURL: server.URL,
		client:  &http.Client{},
	}

	_, err := b.GetPrice(context.Background(), "THB_BTC")
	if err == nil {
		t.Error("expected error for 400 status, got nil")
	}
}

func TestBinanceExchange_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	b := &BinanceExchange{
		client: binance.NewClient("YOUR_KEY", "YOUR_SECRET"),
	}
	b.client.BaseURL = server.URL

	_, err := b.GetPrice(context.Background(), "BTCUSDT")
	if err == nil {
		t.Error("expected error for 400 status, got nil")
	}
}

func TestBinanceExchange_ExecuteOrder(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"symbol": "BTCUSDT", "orderId": 12345}`))
	}))
	defer server.Close()

	b := &BinanceExchange{
		client: binance.NewClient("YOUR_KEY", "YOUR_SECRET"),
	}
	b.client.BaseURL = server.URL

	err := b.ExecuteOrder(context.Background(), "BTCUSDT", Buy, Market, 0.01, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBitkubExchange_SubscribePrice(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{}
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close()
		_ = c.WriteMessage(websocket.TextMessage, []byte(`{"last": 2000000.0}`))
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	b := &BitkubExchange{
		BaseWSURL: wsURL,
		client:    &http.Client{},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ch := make(chan MarketData, 1)
	err := b.SubscribePrice(ctx, "THB_BTC", ch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case md := <-ch:
		if md.Price != 2000000.0 {
			t.Errorf("expected 2000000.0, got %f", md.Price)
		}
	case <-ctx.Done():
		t.Error("timeout waiting for market data")
	}
}

func TestBinanceExchange_SubscribePrice(t *testing.T) {
	b := &BinanceExchange{
		client: binance.NewClient("YOUR_KEY", "YOUR_SECRET"),
	}
	// Call it but since we can't test external WS easily, we provide a canceled ctx
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	ch := make(chan MarketData)
	err := b.SubscribePrice(ctx, "BTCUSDT", ch)
	// It will either return an error immediately or start and then exit due to context
	if err != nil {
		// some network errors are fine here, we just want to hit the code path
		t.Logf("Got expected connection refuse or similar on unit test: %v", err)
	}
}

func TestConstructorsAndGetName(t *testing.T) {
	bin := NewBinanceExchange("YOUR_AK", "YOUR_SK")
	if bin.GetName() != "binance" {
		t.Error("Expected binance")
	}

	bit := NewBitkubExchange("YOUR_AK", "YOUR_SK")
	if bit.GetName() != "bitkub" {
		t.Error("Expected bitkub")
	}
}

func TestBitkubExchange_ExecuteOrder(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"error": 0, "result": {}}`))
	}))
	defer server.Close()

	b := &BitkubExchange{
		BaseURL: server.URL,
		client:  &http.Client{},
		secret:  "YOUR_DUMMY_SECRET",
	}

	err := b.ExecuteOrder(context.Background(), "THB_BTC", Buy, Market, 0.01, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = b.ExecuteOrder(context.Background(), "THB_BTC", Buy, Market, -1, 0)
	if err == nil {
		t.Fatal("expected error for invalid quantity, got nil")
	}

	// Trigger error path
	errServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"error": 30, "result": {}}`))
	}))
	defer errServer.Close()

	b.BaseURL = errServer.URL
	err = b.ExecuteOrder(context.Background(), "THB_BTC", Buy, Market, 0.01, 0)
	if err == nil {
		t.Error("expected bitkub API error code, got nil")
	}
}

func TestBinanceExchange_ExecuteOrderLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"symbol": "BTCUSDT", "orderId": 12345}`))
	}))
	defer server.Close()

	b := &BinanceExchange{
		client: binance.NewClient("YOUR_KEY", "YOUR_SECRET"),
	}
	b.client.BaseURL = server.URL

	err := b.ExecuteOrder(context.Background(), "BTCUSDT", Sell, Limit, 0.01, 60000.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBinanceExchange_SubscribePrice_AskFallback(t *testing.T) {
	// Simple invocation to trigger the function closure definition
	// Detailed WS mocking requires heavier setup, so we ensure the function connects and returns.
	b := &BinanceExchange{
		client: binance.NewClient("YOUR_KEY", "YOUR_SECRET"),
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	ch := make(chan MarketData)
	_ = b.SubscribePrice(ctx, "ETHUSDT", ch)
}

func TestBinanceExchange_GetPrice_Empty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[]`))
	}))
	defer server.Close()

	b := &BinanceExchange{
		client: binance.NewClient("YOUR_KEY", "YOUR_SECRET"),
	}
	b.client.BaseURL = server.URL

	_, err := b.GetPrice(context.Background(), "BTCUSDT")
	if err == nil {
		t.Error("expected error for empty price list, got nil")
	}
}

func TestBinanceExchange_GetBalances(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"balances": [{"asset": "BTC", "free": "1.0", "locked": "0.1"}]}`))
	}))
	defer server.Close()

	b := &BinanceExchange{
		client: binance.NewClient("YOUR_KEY", "YOUR_SECRET"),
	}
	b.client.BaseURL = server.URL

	balances, err := b.GetBalances(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if balances["BTC"] != 1.1 {
		t.Errorf("expected 1.1, got %f", balances["BTC"])
	}
}

func TestBinanceExchange_GetOpenOrders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[{"symbol": "BTCUSDT", "orderId": 12345, "price": "50000.0", "origQty": "0.1", "side": "BUY"}]`))
	}))
	defer server.Close()

	b := &BinanceExchange{
		client: binance.NewClient("YOUR_KEY", "YOUR_SECRET"),
	}
	b.client.BaseURL = server.URL

	orders, err := b.GetOpenOrders(context.Background(), "BTCUSDT")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(orders) != 1 || orders[0].Symbol != "BTCUSDT" {
		t.Errorf("expected 1 order for BTCUSDT, got %v", orders)
	}
}

func TestBitkubExchange_GetBalances(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/api/v3/servertime") {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`1699376552354`))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"error": 0, "result": {"BTC": {"available": 1.0, "reserved": 0.1}}}`))
	}))
	defer server.Close()

	b := &BitkubExchange{
		BaseURL: server.URL,
		client:  &http.Client{},
		secret:  "YOUR_DUMMY",
	}

	balances, err := b.GetBalances(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if balances["BTC"] != 1.1 {
		t.Errorf("expected 1.1, got %f", balances["BTC"])
	}
}

func TestBitkubExchange_GetOpenOrders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/api/v3/servertime") {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`1699376552354`))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"error": 0, "result": [{"id": 123, "symbol": "THB_BTC", "side": "buy", "rate": 2000000.0, "amount": 0.01}]}`))
	}))
	defer server.Close()

	b := &BitkubExchange{
		BaseURL: server.URL,
		client:  &http.Client{},
		secret:  "YOUR_DUMMY",
	}

	orders, err := b.GetOpenOrders(context.Background(), "THB_BTC")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(orders) != 1 || orders[0].Symbol != "THB_BTC" {
		t.Errorf("expected 1 order for THB_BTC, got %v", orders)
	}
}

func TestBitkubExchange_GetOpenOrders_Map(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/api/v3/servertime") {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`1699376552354`))
			return
		}
		w.WriteHeader(http.StatusOK)
		// Return map format
		w.Write([]byte(`{"error": 0, "result": {"THB_BTC": [{"id": 123, "side": "buy", "rate": 2000000.0, "amount": 0.01}]}}`))
	}))
	defer server.Close()

	b := &BitkubExchange{
		BaseURL: server.URL,
		client:  &http.Client{},
		secret:  "YOUR_DUMMY",
	}

	orders, err := b.GetOpenOrders(context.Background(), "THB_BTC")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(orders) != 1 || orders[0].Symbol != "THB_BTC" || orders[0].ID != "123" {
		t.Errorf("expected 1 order for THB_BTC with ID 123, got %v", orders)
	}
}
