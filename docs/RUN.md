# 🚀 Installation & Run Guide

> [!TIP]
> **Developer?** If you intend to contribute code or run the test suite, please follow the [Developer Journey (Onboarding)](./ONBOARDING.md) instead.

## 📋 Prerequisites

- **Go 1.24+**: Download and install from [go.dev](https://go.dev/dl/).
- **Git**: To clone the repository.
- **API Keys**: You'll need API keys from Binance or Bitkub for private trade operations (not required for public ticker polling).

## 🛠️ Installation

1. **Clone the repository**:
   ```bash
   git clone https://github.com/netfirms/niceboy.git
   cd niceboy
   ```

2. **Install dependencies**:
   ```bash
   go mod download
   ```

## ⚙️ Configuration

1. **Create your config file**:
   Copy the example configuration (or let the bot generate it on first run):
   ```bash
   touch config.yaml
   ```

2. **Configure your exchanges**:
   Edit `config.yaml` with your preferred exchange and API credentials:
   ```yaml
   active_exchange: binance
   exchanges:
     binance:
       name: binance
       key: "YOUR_BINANCE_API_KEY"
       secret: "YOUR_BINANCE_SECRET_KEY"
     bitkub:
       name: bitkub
       key: "YOUR_BITKUB_API_KEY"
       secret: "YOUR_BITKUB_SECRET_KEY"
       symbol: "THB_BTC"
   
   # Global Safety Switch
   dry_run: true
   ```

## 🏃 Running the Bot

### Development Mode
To run the bot directly without a separate build step:
```bash
go run cmd/niceboy/main.go
```

### Production Build
To create a high-performance, single binary:
```bash
# Build the binary
go build -o niceboy cmd/niceboy/main.go

# Run the binary
./niceboy
```

## 📊 Sample Output

Upon a successful start, the terminal will render the structured **Bubble Tea / Lipgloss Dashboard**:

```text
  ⚡ niceboy ⚡ (v1.0) [DRY RUN]
  BINANCE : BTCUSDT
  [ DASHBOARD ] [ AUDIT LOGS ]
  ╭───────────────────────────────────╮╭───────────────────────────────────╮
  │ Status:  Connected                ││ Current Signal: BUY               │
  │ Price:   $60420.50                ││ Logic: SMA Crossover (Fast > Slow)│
  │ Trades:  1                        ││                                   │
  ╰───────────────────────────────────╯╰───────────────────────────────────╯
  ╭───────────────────────────────────╮╭───────────────────────────────────╮
  │ PORTFOLIO (USDT)                  ││ OPEN ORDERS                       │
  │ Available: 1,250.00               ││ No active orders.                 │
  │ Locked:    500.00                 ││                                   │
  ╰───────────────────────────────────╯╰───────────────────────────────────╯
  [q:quit] [tab:switch view]
```

---

*Need help? Open an issue on GitHub!*
