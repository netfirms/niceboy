# 🚀 How to Run niceboy

This guide will walk you through setting up and running your `niceboy` trading bot.

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

Upon a successful start, you should see output similar to this:
```text
⚡ niceboy starting...
Active exchange: binance
Using strategy: sma_crossover
Entering trading loop...
[BTCUSDT] sma_crossover Action: WAIT | Reason: Collecting data...
...
🚀 niceboy ready.
```

---

*Need help? Open an issue on GitHub!*
