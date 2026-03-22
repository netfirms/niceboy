package config

import (
	"os"
	"testing"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "Valid Config",
			config: Config{
				ActiveExchange: "binance",
				Exchanges: map[string]ExchangeConfig{
					"binance": {Name: "binance", Symbol: "BTCUSDT"},
				},
			},
			wantErr: false,
		},
		{
			name: "Missing Active Exchange",
			config: Config{
				ActiveExchange: "",
			},
			wantErr: true,
		},
		{
			name: "Missing Exchange Config",
			config: Config{
				ActiveExchange: "bitkub",
				Exchanges: map[string]ExchangeConfig{
					"binance": {Name: "binance", Symbol: "BTCUSDT"},
				},
			},
			wantErr: true,
		},
		{
			name: "Missing Symbol",
			config: Config{
				ActiveExchange: "binance",
				Exchanges: map[string]ExchangeConfig{
					"binance": {Name: "binance"},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.config.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoadConfig_NonExistent(t *testing.T) {
	// Should return default config if file doesn't exist
	cfg, err := LoadConfig("non-existent.yaml")
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if cfg.ActiveExchange != "binance" {
		t.Errorf("expected binance, got %s", cfg.ActiveExchange)
	}
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	tmpFile := "test_invalid.yaml"
	os.WriteFile(tmpFile, []byte("invalid: yaml: :"), 0644)
	defer os.Remove(tmpFile)

	_, err := LoadConfig(tmpFile)
	if err == nil {
		t.Error("expected error for invalid YAML, got nil")
	}
}

func TestConfig_ApplyEnvOverrides(t *testing.T) {
	cfg := &Config{
		Exchanges: map[string]ExchangeConfig{
			"binance": {Name: "binance", Symbol: "BTCUSDT", Key: "old-key"},
		},
	}

	os.Setenv("NICEBOY_BINANCE_KEY", "new-key")
	os.Setenv("NICEBOY_BINANCE_SYMBOL", "ETHUSDT")
	defer os.Unsetenv("NICEBOY_BINANCE_KEY")
	defer os.Unsetenv("NICEBOY_BINANCE_SYMBOL")

	cfg.applyEnvOverrides()

	if cfg.Exchanges["binance"].Key != "new-key" {
		t.Errorf("expected new-key, got %s", cfg.Exchanges["binance"].Key)
	}
	if cfg.Exchanges["binance"].Symbol != "ETHUSDT" {
		t.Errorf("expected ETHUSDT, got %s", cfg.Exchanges["binance"].Symbol)
	}
}
