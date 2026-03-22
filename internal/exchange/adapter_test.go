package exchange

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/adshao/go-binance/v2"
)

func TestBinanceExchange_GetPrice(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"price": "50000.00"}`))
	}))
	defer server.Close()

	// Direct instantiation to bypass production URL
	b := &BinanceExchange{
		client: binance.NewClient("key", "secret"),
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
		w.Write([]byte(`{"result": {"THB_BTC": {"last": 2000000.0}}}`))
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
		t.Errorf("expected 2000000.0, got %f", price)
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
		client: binance.NewClient("key", "secret"),
	}
	b.client.BaseURL = server.URL

	_, err := b.GetPrice(context.Background(), "BTCUSDT")
	if err == nil {
		t.Error("expected error for 400 status, got nil")
	}
}
