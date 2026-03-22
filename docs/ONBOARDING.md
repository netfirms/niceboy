# 🚀 Developer Onboarding Guide

> [!NOTE]
> **Just want to run the bot?** Follow the [User Journey (Run Guide)](./RUN.md) for simple deployment steps.

Welcome to the `niceboy` project! This document will help you set up your local development environment and get started with the codebase.

## 📋 Prerequisites
- **Go 1.24+**: Ensure you have the latest Go version installed.
- **Git**: For version control.
- **Make**: For running our orchestration suite.

## 🛠️ Getting Started

### 1. Clone & Tidy
```bash
git clone https://github.com/netfirms/niceboy.git
cd niceboy
make tidy
```

### 2. Configure Environment
Copy the example configuration:
```bash
cp config.example.yaml config.yaml
```
*Note: Never commit your `config.yaml` to Git. Use environment variables for secrets.*

## ⚙️ Development Workflow

The project uses a `Makefile` to simplify common tasks.

| Command | Description |
|---------|-------------|
| `make build` | Compiles the binary to `./niceboy` |
| `make test` | Runs the entire unit testing suite |
| `make coverage` | Runs tests and shows a coverage report (Target: 80%+) |
| `make lint` | Runs `golangci-lint` or `go vet` |
| `make run` | Builds and starts the bot in TUI mode |
| `make clean` | Removes binaries and log files |

## 🧪 Testing Standards
We maintain a strict **80% statement coverage** policy for core packages (`internal/config`, `internal/strategy`, `internal/exchange`).
- Add tests for every new feature.
- Use mocks for any external API interaction (refer to `internal/exchange/adapter_test.go`).
- Refer to [docs/TESTING.md](./TESTING.md) for deeper details.

## 🛡️ Security Protocol
- **No Secrets in Code**: Use environment variables (`NICEBOY_BINANCE_KEY`, etc.) for credentials.
- **Scrubbing**: Ensure no sensitive data is printed to logs (see [docs/SPECIFICATION.md](./SPECIFICATION.md#44-security)).
- **Git Hygiene**: Check your staged files to ensure no `.log` or `.yaml` secrets are included.

## 🏗️ Architecture Overview
Before submitting a PR, please read:
- [ARCHITECTURE.md](../ARCHITECTURE.md): The high-level design philosophy.
- [docs/SPECIFICATION.md](./SPECIFICATION.md): The technical "Source of Truth".

**Key Technical Nuances for Contributors:**
- **UI Tweaks**: Any graphical additions must utilize the `charmbracelet/lipgloss` styling engine inside `internal/ui`.
- **Latency**: Strategy additions (`OnMarketData`) flow directly from the WebSocket loop, so they must be allocation-free to prevent GC pauses.
- **Execution**: Mock out `ExecuteOrder` calls in `adapter_test.go` when adding new exchange logic.

Happy Coding! ⚡
