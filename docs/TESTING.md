# Testing Suite Guide

This document describes how to ensure the reliability and security of the `niceboy` bot using the automated testing suite.

## 🎯 Testing Goals
- **Maintain 80%+ Coverage**: All core packages (`internal/config`, `internal/strategy`, `internal/exchange`) must hit this threshold.
- **Mocked Integration**: No live network calls allowed in unit tests.
- **Deterministic Logic**: Strategy signals must be verifiable with synthetic market data sequences.

## 🛠️ Running Tests

### Standard Run
Run all tests in the project:
```bash
make test
```

### Coverage Report
Check statement coverage across all packages:
```bash
make coverage
```

## 🧪 Implementation Details

### Strategy Testing
Located in `internal/strategy/*_test.go`. Uses table-driven tests to feed `MarketData` into strategies and verify the resulting `Signal`.

### Exchange Mocking
Located in `internal/exchange/adapter_test.go`. Uses `httptest` to mock real exchange API responses, ensuring the parsing logic is robust against API changes or malformed JSON.

### Configuration Validation
Located in `internal/config/config_test.go`. Verifies that the bot correctly handles missing configuration files, invalid YAML syntax, and peak-precedence environment variables.

## 🚀 Future Enhancements
- [ ] **Fuzz Testing**: Add Go fuzzing for strategy input parameters.
- [ ] **Integration Tests**: Add a container-based integration test for real-time WebSocket connectivity.
