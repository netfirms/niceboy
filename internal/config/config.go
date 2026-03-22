package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type ExchangeConfig struct {
	Name   string `yaml:"name"`
	Key    string `yaml:"key"`
	Secret string `yaml:"secret"`
	Symbol string `yaml:"symbol"` // e.g. "BTCUSDT" or "THB_BTC"
}

type Config struct {
	ActiveExchange     string                    `yaml:"active_exchange"`
	Strategy           string                    `yaml:"strategy"`
	DryRun             bool                      `yaml:"dry_run"`
	StrategyParameters map[string]interface{}    `yaml:"strategy_parameters"`
	Exchanges          map[string]ExchangeConfig `yaml:"exchanges"`
}

func LoadConfig(path string) (*Config, error) {
	// For now, return a default config if file doesn't exist
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return &Config{
			ActiveExchange: "binance",
			Strategy:       "sma_crossover",
			DryRun:         false,
			StrategyParameters: map[string]interface{}{
				"short_period": 5,
				"long_period":  10,
			},
			Exchanges: map[string]ExchangeConfig{
				"binance": {Name: "binance", Symbol: "BTCUSDT"},
				"bitkub":  {Name: "bitkub", Symbol: "THB_BTC"},
			},
		}, nil
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var cfg Config
	decoder := yaml.NewDecoder(f)
	err = decoder.Decode(&cfg)
	if err != nil {
		return nil, err
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	// Apply Environment Variable Overrides
	cfg.applyEnvOverrides()

	return &cfg, nil
}

func (c *Config) applyEnvOverrides() {
	for name, exch := range c.Exchanges {
		// e.g., NICEBOY_BINANCE_KEY
		prefix := fmt.Sprintf("NICEBOY_%s_", strings.ToUpper(name))

		if envKey := os.Getenv(prefix + "KEY"); envKey != "" {
			exch.Key = envKey
		}
		if envSecret := os.Getenv(prefix + "SECRET"); envSecret != "" {
			exch.Secret = envSecret
		}
		if envSymbol := os.Getenv(prefix + "SYMBOL"); envSymbol != "" {
			exch.Symbol = envSymbol
		}

		c.Exchanges[name] = exch
	}
}

func (c *Config) Validate() error {
	if c.ActiveExchange == "" {
		return fmt.Errorf("active_exchange must be specified")
	}

	exch, ok := c.Exchanges[c.ActiveExchange]
	if !ok {
		return fmt.Errorf("active exchange %s has no configuration", c.ActiveExchange)
	}

	if exch.Symbol == "" {
		return fmt.Errorf("symbol must be specified for exchange %s", c.ActiveExchange)
	}

	return nil
}
