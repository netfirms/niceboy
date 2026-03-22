# ⚡ niceboy

> A low-footprint, high-efficiency console trading bot designed for performance and simplicity.

`niceboy` is built for traders who value speed, reliability, and minimal resource usage. It provides a robust foundation for executing automated trading strategies directly from your terminal.

## 🔄 How it Works

```mermaid
sequenceDiagram
    participant User
    participant Config as config.yaml
    participant Bot as niceboy Core
    participant Exch as Exchange
    participant UI as Console TUI

    User->>Config: 1. Set API Keys & Choice
    User->>Bot: 2. Launch Bot
    Bot->>Exch: 3. Fetch Market Data
    Bot->>Bot: 4. Run Strategy
    Bot->>UI: 5. Update Dashboard
```

## ✨ Features

- **🚀 Low Footprint**: Optimized Go core with sub-10MB memory usage.
- **🖥️ Console-First TUI**: Interactive terminal interface powered by Bubble Tea.
- **🔌 Modular Architecture**: Plug-and-play strategies and unified exchange adapters.
- **🛡️ Secure & Resilient**: Local-first security with recovery-guarded trading loops.
- **📊 Structured Auditing**: Dual-output logging (Console + JSON) for full traceability.

## 🛠️ Technology Stack

- **Language**: [Go 1.24+](https://go.dev/) (Chosen for its official SDK support and efficient concurrency).
- **Logging**: [zerolog](https://github.com/rs/zerolog) (High-performance, structured JSON, persistent audit trail).
- **Exchange Integration**: Official SDKs for Binance and Bitkub via a unified runtime adapter.
- **UI Framework**: [Bubble Tea](https://github.com/charmbracelet/bubbletea) (Terminal User Interface).

## 🧭 Choose Your Journey

Whether you are looking to run the bot or build upon it, follow the path that fits your role:

### 👤 The User Journey (Run the Bot)
*Goal: Deploy `niceboy` and start trading in under 5 minutes.*

1. **Quick Start**: [Install & Run](./docs/RUN.md)
2. **Configuration**: [Setting up Keys & Symbols](./docs/RUN.md#configuration)
3. **Management**: [Running Multiple Instances](./docs/RUN.md#multi-instance-support)

### 💻 The Developer Journey (Build the Bot)
*Goal: Setup the local dev environment and contribute core logic.*

1. **Onboarding**: [Setup & Hello World](./docs/ONBOARDING.md)
2. **Architecture**: [Understand the 5 Pillars](./ARCHITECTURE.md)
3. **Execution**: [Testing & Coverage Suite](./docs/TESTING.md)

---

## ⚡ Quick Start (User Path)

1. **Install Go**: Ensure Go 1.24+ is installed.
2. **Cloning**:
   ```bash
   git clone https://github.com/netfirms/niceboy.git
   cd niceboy
   ```
3. **Config Setup**:
   ```bash
   cp config.example.yaml config.yaml
   # Edit config.yaml with your API keys
   ```
4. **Launch TUI**:
   ```bash
   go run cmd/niceboy/main.go
   ```

## 👯 Running Multiple Instances

`niceboy` supports running multiple independent instances on the same machine. Use CLI flags to specify unique configurations and log files:

```bash
# Instance 1: Binance
go run cmd/niceboy/main.go -config binance_config.yaml -log binance.log

# Instance 2: Bitkub
go run cmd/niceboy/main.go -config bitkub_config.yaml -log bitkub.log
```

For detailed setup, configuration, and production build instructions, see the [**Run Guide (docs/RUN.md)**](docs/RUN.md).

## 📜 Documentation

- [Architecture Overview](ARCHITECTURE.md)
- [Design Goals](GOALS.md)
- [Research: Go vs Rust](RESEARCH_RESULTS.md)
- [Configuration Guide](CONFIG.md)

## ⚖️ License

`niceboy` is released under the **MIT License**. See [LICENSE](LICENSE) for more details.

## 🤝 Contributing

Contributions are what make the open-source community such an amazing place to learn, inspire, and create. Any contributions you make are **greatly appreciated**. Please see our [Contributing Guide](CONTRIBUTING.md) for more details.

---

*Built with ❤️ for the trading community.*
