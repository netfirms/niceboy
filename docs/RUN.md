# 🚀 Installation & Run Guide

`niceboy` is distributed as a single, lightweight binary. Choose the installation method that fits your platform.

## 📥 Installation

### 🍏 macOS (Recommended)
The easiest way to install on macOS is via Homebrew:
```bash
brew install netfirms/niceboy/niceboy
```

### 🐧 Linux
1. Download the latest `niceboy_Linux_x86_64.tar.gz` from [GitHub Releases](https://github.com/netfirms/niceboy/releases).
2. Extract and move to your path:
   ```bash
   tar -xf niceboy_Linux_x86_64.tar.gz
   chmod +x niceboy
   sudo mv niceboy /usr/local/bin/
   ```

### 🪟 Windows
1. Download `niceboy_Windows_x86_64.zip` from [GitHub Releases](https://github.com/netfirms/niceboy/releases).
2. Extract the `.zip` file.
3. Open **PowerShell** or **Command Prompt** in the folder and run:
   ```powershell
   .\niceboy.exe
   ```

## ⚙️ Configuration

`niceboy` requires a `config.yaml` file to run. On the first launch, if no config is found, the bot will start an **Interactive Setup Wizard** to help you generate one.

### Manual Setup
You can also manually create `config.yaml` using the provided templates:
- [Binance Example](file:///Users/taweechai/Documents/pvt/niceboy/binance.example.yaml)
- [Bitkub Example](file:///Users/taweechai/Documents/pvt/niceboy/bitkub.example.yaml)

```bash
# Example: Using the Binance template
cp binance.example.yaml config.yaml
```

## 🏃 Running the Bot

### Basic Execution
```bash
niceboy
```

### Multi-Instance Support
Run multiple bots for different pairs or exchanges by specifying unique config and log files:
```bash
# Instance 1: BTC on Binance
niceboy -config binance_btc.yaml -log btc.log

# Instance 2: ETH on Bitkub
niceboy -config bitkub_eth.yaml -log eth.log
```

### Docker Execution
```bash
docker run -it --rm \
  -v $(pwd)/config.yaml:/app/config.yaml \
  netfirms/niceboy:latest
```

## 🎮 TUI Controls

| Key | Action |
|-----|--------|
| `Tab` | Switch between Dashboard and Audit Logs |
| `k` | **EMERGENCY KILL SWITCH** (Cancels all orders & flattens position) |
| `b` | Manual Force Buy (Tactical) |
| `s` | Manual Force Sell (Tactical) |
| `q` | Quit Safely |

---
*Need help? Open an issue on [GitHub](https://github.com/netfirms/niceboy/issues).*
