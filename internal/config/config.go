package config

import (
	"fmt"
	"os"
	"strings"
	"bufio"

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
	DatabasePath       string                    `yaml:"database_path"`
	OrderQuantity      float64                   `yaml:"order_quantity"`
	SlippagePct        float64                   `yaml:"slippage_pct"`
	StrategyParameters map[string]interface{}    `yaml:"strategy_parameters"`
	Exchanges          map[string]ExchangeConfig `yaml:"exchanges"`
}

func LoadConfig(path string) (*Config, error) {
	// For now, return a default config if file doesn't exist
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return &Config{
			ActiveExchange: "binance",
			Strategy:      "sma_crossover",
			DryRun:        false,
			DatabasePath:  "niceboy.db",
			OrderQuantity: 0.01,
			SlippagePct:   0.5, // 0.5% default
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

	if cfg.DatabasePath == "" {
		cfg.DatabasePath = "niceboy.db"
	}

	if cfg.OrderQuantity == 0 {
		cfg.OrderQuantity = 0.01 // Default fallback
	}

	if cfg.SlippagePct == 0 {
		cfg.SlippagePct = 0.5 // Default 0.5%
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

func (c Config) Validate() error {
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

func (c Config) NeedsSetup() bool {
	exch, ok := c.Exchanges[c.ActiveExchange]
	if !ok {
		return true
	}
	return exch.Key == "" || exch.Secret == ""
}

func (c *Config) SaveConfig(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func (c *Config) InteractiveSetup() error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("\n🌟 Welcome to niceboy! Let's set up your trading bot.")
	fmt.Println("--------------------------------------------------")

	// 1. Choose Exchange
	fmt.Printf("Choose Exchange (binance/bitkub) [%s]: ", c.ActiveExchange)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input != "" {
		c.ActiveExchange = strings.ToLower(input)
	}

	// 2. Initialize exchange config if missing
	if c.Exchanges == nil {
		c.Exchanges = make(map[string]ExchangeConfig)
	}
	exch := c.Exchanges[c.ActiveExchange]
	exch.Name = c.ActiveExchange

	// 3. Set Symbol
	defaultSym := "BTCUSDT"
	if c.ActiveExchange == "bitkub" {
		defaultSym = "THB_BTC"
	}
	if exch.Symbol != "" {
		defaultSym = exch.Symbol
	}
	fmt.Printf("Trading Symbol [%s]: ", defaultSym)
	input, _ = reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input != "" {
		exch.Symbol = input
	} else {
		exch.Symbol = defaultSym
	}

	// 4. API Key
	fmt.Print("API Key: ")
	input, _ = reader.ReadString('\n')
	exch.Key = strings.TrimSpace(input)

	// 5. API Secret
	fmt.Print("API Secret: ")
	input, _ = reader.ReadString('\n')
	exch.Secret = strings.TrimSpace(input)

	c.Exchanges[c.ActiveExchange] = exch

	// 6. Mode
	fmt.Printf("Enable Dry Run (Simulator) (y/n) [y]: ")
	input, _ = reader.ReadString('\n')
	input = strings.ToLower(strings.TrimSpace(input))
	if input == "n" {
		c.DryRun = false
	} else {
		c.DryRun = true
	}

	fmt.Println("\n✅ Setup complete! Saving configuration...")
	return nil
}
