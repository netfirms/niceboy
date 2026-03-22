# 🔍 Gap Analysis: niceboy

This document identifies technical and architectural gaps that should be addressed to ensure the bot is "production-ready" for an open-source audience.

## 1. Error Handling & Resilience 🛡️
- **Current State**: Basic `log.Fatalf` on startup and simple error logging in the main loop.
- **Gap**: No automatic retries for intermittent network errors in `GetPrice`. `SubscribePrice` is only a placeholder.
- **Propsoed Fix**:
  - Implement a basic retry mechanism (exponential backoff) in the exchange integration layer.
  - Finalize the WebSocket implementation for real-time data to avoid REST polling limitations.

## 2. Configuration Validation ⚙️
- **Current State**: Basic YAML decoding. `main.go` checks if the active exchange exists.
- **Gap**: No schema validation. Missing API keys are only discovered at runtime during specific operations.
- **Proposed Fix**:
  - Add a `Validate()` method to the `Config` struct.
  - Ensure that if an exchange is set as "active", its required fields (like symbol naming convention) are present.

## 3. Structured Logging 📊
- **Current State**: Mixed use of `fmt.Printf` and `log`.
- **Gap**: Difficult to parse logs for automated monitoring or debugging in production.
- **Proposed Fix**:
  - Integrate `github.com/rs/zerolog` (already a transient dependency) into the core kernel.
  - Add request IDs or context to log entries.

## 4. Symbology & Market Data 📈
- **Current State**: Hardcoded symbol switching in `main.go` (`BTCUSDT` vs `THB_BTC`).
- **Gap**: Inflexible when adding new symbols or exchanges with different naming conventions.
- **Proposed Fix**:
  - Add a `TargetSymbol` field to each `ExchangeConfig`.
  - Let the adapter handle the exchange-specific normalization.

## 5. Strategy Parameterization 🧠
- **Current State**: `SMACrossover` has hardcoded periods.
- **Gap**: Users cannot adjust strategy parameters without recompiling.
- **Proposed Fix**:
  - Update the `Registry` to support passing a map of parameters to strategy factories.
  - Allow these parameters to be defined in `config.yaml`.

---
*Addressing these gaps will significantly improve the "Instituional Grade" quality of the bot.*
