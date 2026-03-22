# 📐 Technical Specification: niceboy

This document serves as the formal "Source of Truth" for the `niceboy` project's technical requirements and implementation details.

## 1. Runtime & Performance Targets
- **Language**: Go 1.24+ (Strictly required for latest goroutine optimizations).
- **Binary**: Must produce a single, statically-linked binary (AOT) with no runtime dependencies.
- **Platform**: Native support for `darwin/arm64`, `linux/amd64`, and `windows/amd64`.
- **Memory Footprint**: < 10MB idle, < 25MB under heavy load (10+ symbols).
- **Hot Path Latency**: < 5ms for signal generation after data receipt.

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
- `SubscribePrice(ctx, symbol, ch)`: Real-time stream (WebSocket). **MUST** implement internal self-healing with exponential backoff.
- `ExecuteOrder(ctx, symbol, side, type, qty, price) error`: Live trade execution.
- `GetBalances(ctx) (map[string]float64, error)`: Aggregated account holdings.
- `GetOpenOrders(ctx, symbol) ([]Order, error)`: Staged market instructions.

### 3.2 Strategy Interface
- `GetName() string`: Unique identifier for the registry.
- `OnMarketData(MarketData) Signal`: Stateful logic core. Strategies **SHOULD** track `inPosition` and `entryPrice` for risk management.

## 4. Operational Requirements

### 4.1 Error Resilience
- **Timeouts**: Every external network call MUST have a context timeout (default 10s).
- **Recovery**: The main trading loop MUST wrap every iteration in a `panic(recover)` handler to prevent bot death on unanticipated SDK errors.

### 4.2 Local Auditing
- **Format**: All operational logs must be in **structured JSON** via `zerolog`.
- **Output**: Multi-output to `stderr` (console-formatted) and a user-defined `.log` file.
- **Secret Scrubbing**: All fields named `Key`, `Secret`, or any raw credential MUST be masked in the audit logs (e.g., `key="***"`).

### 4.4 Trading Logic & Profitability
- **Precision**: Calculations for indicators must use `float64` or decimal libraries to ensure sub-satoshi accuracy.
- **Context Awareness**: Strategies must receive the latest order book depth (if supported) to calculate effective exit prices.

## 5. Security & Isolation
- **Credential Precedence**: Runtime environment variables (`NICEBOY_{EXCH}_{FIELD}`) ALWAYS override static entries in `config.yaml`.
- **Isolation**: Each bot instance must operate in its own process space with unique log and config handles.

## 6. Automated Guardrails (Pre-Commit)
- **Secret Scanning**: Staged logic MUST be scanned for high-entropy strings and credential patterns (e.g., `key:`, `secret:`) that lack placeholder prefixes.
- **QA Enforcement**: All unit tests MUST pass locally before a commit is approved by the Git hook.
