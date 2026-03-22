# 🔄 How niceboy Works

This diagram illustrates the typical lifecycle of a `niceboy` session from a user's perspective.

```mermaid
sequenceDiagram
    participant User
    participant Config as config.yaml
    participant Bot as niceboy Core
    participant Exch as Exchange (Binance/Bitkub)
    participant UI as Console TUI

    User->>Config: 1. Set API Keys & Choice
    User->>Bot: 2. Launch Bot
    Bot->>Config: 3. Load Settings
    Bot->>Exch: 4. Connect WebSocket Stream
    Exch-->>Bot: 5. Real-time Market Data
    Bot->>Bot: 6. Run Strategy (e.g. SMA)
    Bot->>UI: 7. Update Dashboard
    alt Trade Signal Found
        Bot->>Exch: 8. Execute Market/Limit Order
        Exch-->>Bot: 9. Execution Report
        Bot->>UI: 10. Update Trade Count & Logs
    end
    UI-->>User: Visual Feedback
```

## 🛠️ Step-by-Step Breakdown

1.  **Configuration**: User provides API credentials and selects the active exchange and strategy in `config.yaml`.
2.  **Lifecycle Start**: The bot initializes the selected exchange adapter and strategy.
3.  **Real-time Loop**: Market data is streamed instantly via WebSocket connections (e.g., Binance `WsBookTickerServe`).
4.  **Intelligence**: The strategy engine processes the data and generates signals (BUY/SELL).
5.  **Execution**: The execution engine handles exact order payload structure and fires trades based on strategy cues.
6.  **Visualization**: Everything is presented in a clean, interactive TUI.
