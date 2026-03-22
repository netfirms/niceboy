# 📐 Technical Specification: niceboy

This document serves as the formal "Source of Truth" for the `niceboy` project's technical requirements and implementation details.

## 1. Runtime Environment
- **Language**: Go 1.24+ (Strictly required for latest goroutine optimizations).
- **Binary**: Must produce a single, statically-linked binary (AOT) with no runtime dependencies.
- **Platform**: Native support for `darwin/arm64`, `linux/amd64`, and `windows/amd64`.

## 2. Core Data Structures

### 2.1 MarketData
```go
type MarketData struct {
    Symbol string  // Normalized (e.g., BTCUSDT)
    Price  float64 // Latest ticker price
    Time   int64   // Unix timestamp (milliseconds)
}
```

### 2.2 Signal
```go
type Signal struct {
    Type   SignalType // BUY, SELL, or WAIT
    Symbol string
    Price  float64
    Reason string     // Human-readable rationale
}
```

## 3. Modular Interfaces

### 3.1 Exchange Adapter
Adapters must implement the following:
- `GetName() string`: Returns the ID of the exchange.
- `GetPrice(ctx, symbol) (float64, error)`: Unicast price fetch.
- `SubscribePrice(ctx, symbol, ch)`: Real-time stream (WebSocket).

### 3.2 Strategy Interface
- `GetName() string`: Unique identifier for the registry.
- `OnMarketData(MarketData) Signal`: Stateless (or stateful) logic core.

## 4. Operational Requirements

### 4.1 Error Resilience
- **Timeouts**: Every external network call MUST have a context timeout (default 10s).
- **Recovery**: The main trading loop MUST wrap every iteration in a `panic(recover)` handler to prevent bot death on unanticipated SDK errors.

### 4.2 Local Auditing
- **Format**: All operational logs must be in **structured JSON** via `zerolog`.
- **Output**: Multi-output to `stderr` (console-formatted) and a user-defined `.log` file.
- **Secret Scrubbing**: All fields named `Key`, `Secret`, or any raw credential MUST be masked in the audit logs (e.g., `key="***"`).

### 4.3 Configuration & Secrets
- **Credential Precedence**: Runtime environment variables (`NICEBOY_{EXCH}_{FIELD}`) ALWAYS override static entries in `config.yaml`.
- **Validation**: Path validation is mandatory before the bot enters the "Ready" state.

## 5. Performance Targets
- **Memory Footprint**: < 10MB idle, < 25MB under heavy load (10+ symbols).
- **Hot Path Latency**: < 5ms for signal generation after data receipt.
