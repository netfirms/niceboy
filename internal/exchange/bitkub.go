package exchange

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type BitkubExchange struct {
	BaseURL   string
	BaseWSURL string
	client    *http.Client
	apiKey    string
	secret    string
}

func NewBitkubExchange(apiKey, secretKey string) *BitkubExchange {
	return &BitkubExchange{
		BaseURL:   "https://api.bitkub.com",
		BaseWSURL: "wss://api.bitkub.com/websocket-api",
		client:    &http.Client{Timeout: 10 * time.Second},
		apiKey:    apiKey,
		secret:    secretKey,
	}
}

func (b *BitkubExchange) getServerTime() (int64, error) {
	resp, err := b.client.Get(b.BaseURL + "/api/v3/servertime")
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	var ts int64
	if err := json.NewDecoder(resp.Body).Decode(&ts); err != nil {
		return 0, err
	}
	return ts, nil
}

func (b *BitkubExchange) GetName() string {
	return "bitkub"
}

func (b *BitkubExchange) normalizeSymbol(symbol string) string {
	s := strings.ToUpper(symbol)
	if strings.HasPrefix(s, "THB_") {
		// Convert THB_BTC to BTC_THB
		parts := strings.Split(s, "_")
		if len(parts) == 2 {
			return parts[1] + "_" + parts[0]
		}
	}
	return s
}

func (b *BitkubExchange) denormalizeSymbol(symbol string) string {
	s := strings.ToUpper(symbol)
	if strings.HasSuffix(s, "_THB") {
		// Convert BTC_THB to THB_BTC
		parts := strings.Split(s, "_")
		if len(parts) == 2 {
			return parts[1] + "_" + parts[0]
		}
	}
	return s
}

func (b *BitkubExchange) GetPrice(ctx context.Context, symbol string) (float64, error) {
	normSym := b.normalizeSymbol(symbol)
	url := fmt.Sprintf("%s/api/v3/market/ticker?sym=%s", b.BaseURL, normSym)
	resp, err := b.client.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("bitkub ticker failed, status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	// Bitkub V3 Ticker can return directly or wrapped in an envelope
	var envelope struct {
		Error  int `json:"error"`
		Result []struct {
			Last float64 `json:"last"`
		} `json:"result"`
	}

	var directData []struct {
		Last float64 `json:"last"`
	}

	if err := json.Unmarshal(body, &envelope); err == nil && len(envelope.Result) > 0 {
		return envelope.Result[0].Last, nil
	} else if err := json.Unmarshal(body, &directData); err == nil && len(directData) > 0 {
		return directData[0].Last, nil
	}

	return 0, fmt.Errorf("failed to parse Bitkub ticker response: %s", string(body))
}
func (b *BitkubExchange) SubscribePrice(ctx context.Context, symbol string, ch chan<- MarketData) error {
	// WS URL is just the base
	wsURL := b.BaseWSURL
	// Bitkub WS topic uses quote_base (e.g. thb_btc)
	topic := fmt.Sprintf("market.ticker.%s", strings.ToLower(symbol))

	go func() {
		backoff := time.Second
		for {
			select {
			case <-ctx.Done():
				return
			default:
				c, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
				if err != nil {
					time.Sleep(backoff)
					if backoff < 30*time.Second {
						backoff *= 2
					}
					continue
				}

				// Reset backoff on successful connection
				backoff = time.Second

				// Subscribe via message
				subMsg := map[string]interface{}{
					"event":   "subscribe",
					"streams": []string{topic},
				}
				if err := c.WriteJSON(subMsg); err != nil {
					c.Close()
					continue
				}

				for {
					_, message, err := c.ReadMessage()
					if err != nil {
						c.Close()
						break // Trigger reconnect
					}

					var raw struct {
						Stream string `json:"stream"`
						Data   struct {
							Last       float64 `json:"last"`
							BaseVolume float64 `json:"base_volume"`
						} `json:"data"`
					}
					if err := json.Unmarshal(message, &raw); err == nil && raw.Data.Last > 0 {
						select {
						case ch <- MarketData{
							Symbol: symbol,
							Price:  raw.Data.Last,
							Volume: raw.Data.BaseVolume,
							Time:   time.Now().UnixNano() / 1e6,
						}:
						case <-ctx.Done():
							c.Close()
							return
						}
					}
				}
			}
		}
	}()
	return nil
}

func (b *BitkubExchange) SubscribeKlines(ctx context.Context, symbol string, interval string, ch chan<- Kline) error {
	return fmt.Errorf("SubscribeKlines not implemented for Bitkub")
}

func (b *BitkubExchange) GetKlines(ctx context.Context, symbol string, interval string, limit int) ([]Kline, error) {
	return nil, fmt.Errorf("GetKlines not implemented for Bitkub")
}

func (b *BitkubExchange) ExecuteOrder(ctx context.Context, symbol string, side OrderSide, orderType OrderType, quantity float64, price float64) error {
	if quantity <= 0 {
		return fmt.Errorf("invalid quantity: %f", quantity)
	}

	endpoint := "/api/v3/market/place-bid"
	if side == Sell {
		endpoint = "/api/v3/market/place-ask"
	}

	url := b.BaseURL + endpoint

	typ := "market"
	if orderType == Limit {
		typ = "limit"
	}

	normSym := b.normalizeSymbol(symbol)
	
	// Ensure numeric precision is respected in the JSON payload
	// Although Bitkub accepts floats, many institutional APIs prefer strings or specific decimal counts.
	// We'll use the provided floats directly but they are pre-formatted in main.go.
	payload := map[string]interface{}{
		"sym":  normSym,
		"amt":  quantity,
		"rat":  price,
		"type": typ,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	ts, err := b.getServerTime()
	if err != nil {
		ts = time.Now().UnixMilli() // Ensure ms
	}

	sigBody := ""
	if len(payloadBytes) > 0 {
		sigBody = string(payloadBytes)
	}
	sigStr := fmt.Sprintf("%dPOST%s%s", ts, endpoint, sigBody)

	mac := hmac.New(sha256.New, []byte(b.secret))
	mac.Write([]byte(sigStr))
	sig := hex.EncodeToString(mac.Sum(nil))

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-BTK-APIKEY", b.apiKey)
	req.Header.Set("X-BTK-TIMESTAMP", fmt.Sprintf("%d", ts))
	req.Header.Set("X-BTK-SIGN", sig)

	resp, err := b.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bitkub order execution failed, status: %d", resp.StatusCode)
	}

	// In a real application we would unmarshal the API response to check for logical error codes (e.g. error: 0 is success)
	// Example: {"error": 0, "result": {...}}
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err == nil {
		if errCode, ok := result["error"].(float64); ok && errCode != 0 {
			return fmt.Errorf("bitkub API error code: %.0f", errCode)
		}
	}

	return nil
}

func (b *BitkubExchange) GetBalances(ctx context.Context) (map[string]float64, error) {
	endpoint := "/api/v3/market/balances"

	ts, err := b.getServerTime()
	if err != nil {
		ts = time.Now().UnixMilli()
	}
	// V3 spec requires timestamp + method + endpoint + payload (even if empty)
	sigStr := fmt.Sprintf("%dPOST%s", ts, endpoint)

	mac := hmac.New(sha256.New, []byte(b.secret))
	mac.Write([]byte(sigStr))
	sig := hex.EncodeToString(mac.Sum(nil))

	req, err := http.NewRequestWithContext(ctx, "POST", b.BaseURL+endpoint, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-BTK-APIKEY", b.apiKey)
	req.Header.Set("X-BTK-TIMESTAMP", fmt.Sprintf("%d", ts))
	req.Header.Set("X-BTK-SIGN", sig)

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bitkub api failed, status: %d", resp.StatusCode)
	}

	var data struct {
		Error  int `json:"error"`
		Result map[string]struct {
			Available float64 `json:"available"`
			Reserved  float64 `json:"reserved"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	if data.Error != 0 {
		return nil, fmt.Errorf("bitkub error code: %d", data.Error)
	}

	balances := make(map[string]float64)
	for asset, detail := range data.Result {
		total := detail.Available + detail.Reserved
		if total > 0 {
			balances[asset] = total
		}
	}
	return balances, nil
}

func (b *BitkubExchange) GetOpenOrders(ctx context.Context, symbol string) ([]Order, error) {
	endpoint := "/api/v3/market/my-open-orders"
	normSym := b.normalizeSymbol(symbol)
	query := "sym=" + normSym

	ts, err := b.getServerTime()
	if err != nil {
		ts = time.Now().UnixNano() / 1e6 // Fallback
	}

	// Bitkub V3 GET signature often includes the '?' in the payload.
	// But let's try to be very precise.
	sigStr := fmt.Sprintf("%dGET%s?%s", ts, endpoint, query)

	mac := hmac.New(sha256.New, []byte(b.secret))
	mac.Write([]byte(sigStr))
	sig := hex.EncodeToString(mac.Sum(nil))

	url := b.BaseURL + endpoint + "?" + query
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	// Removing Content-Type for GET as it might cause issues on some servers
	req.Header.Set("X-BTK-APIKEY", b.apiKey)
	req.Header.Set("X-BTK-TIMESTAMP", fmt.Sprintf("%d", ts))
	req.Header.Set("X-BTK-SIGN", sig)

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bitkub api failed, status: %d, body: %s", resp.StatusCode, string(body))
	}

	var envelope struct {
		Error  int             `json:"error"`
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("bitkub envelope unmarshal error: %v", err)
	}

	if envelope.Error != 0 {
		return nil, fmt.Errorf("bitkub error code: %d", envelope.Error)
	}

	trimmed := bytes.TrimSpace(envelope.Result)
	if len(trimmed) == 0 {
		return nil, nil
	}

	var res []Order

	// Case 1: Result is an array []
	if trimmed[0] == '[' {
		var orders []struct {
			ID     interface{} `json:"id"`
			Symbol string      `json:"symbol"`
			Side   string      `json:"side"`
			Rate   interface{} `json:"rate"`
			Amount interface{} `json:"amount"`
		}
		if err := json.Unmarshal(trimmed, &orders); err != nil {
			return nil, fmt.Errorf("bitkub array unmarshal error: %v (body: %s)", err, string(body))
		}
		for _, o := range orders {
			res = append(res, b.parseOrder(o.ID, o.Symbol, o.Side, o.Rate, o.Amount))
		}
		return res, nil
	}

	// Case 2: Result is a map {}
	if trimmed[0] == '{' {
		var orderMap map[string][]struct {
			ID     interface{} `json:"id"`
			Side   string      `json:"side"`
			Rate   interface{} `json:"rate"`
			Amount interface{} `json:"amount"`
		}
		if err := json.Unmarshal(trimmed, &orderMap); err != nil {
			// Maybe it's not a map of arrays? Try map of single objects just in case.
			var singleOrderMap map[string]struct {
				ID     interface{} `json:"id"`
				Side   string      `json:"side"`
				Rate   interface{} `json:"rate"`
				Amount interface{} `json:"amount"`
			}
			if err2 := json.Unmarshal(trimmed, &singleOrderMap); err2 == nil {
				for sym, o := range singleOrderMap {
					res = append(res, b.parseOrder(o.ID, sym, o.Side, o.Rate, o.Amount))
				}
				return res, nil
			}
			return nil, fmt.Errorf("bitkub map unmarshal error: %v (body: %s)", err, string(body))
		}
		for sym, orders := range orderMap {
			for _, o := range orders {
				res = append(res, b.parseOrder(o.ID, sym, o.Side, o.Rate, o.Amount))
			}
		}
		return res, nil
	}

	return nil, fmt.Errorf("bitkub unexpected result format: %s", string(trimmed))
}

// Helper to parse polymorphic types from Bitkub
func (b *BitkubExchange) parseOrder(id interface{}, symbol, sideStr string, rate, amount interface{}) Order {
	var r, a float64
	switch v := rate.(type) {
	case float64:
		r = v
	case string:
		fmt.Sscanf(v, "%f", &r)
	}
	switch v := amount.(type) {
	case float64:
		a = v
	case string:
		fmt.Sscanf(v, "%f", &a)
	}

	side := Buy
	if strings.ToLower(sideStr) == "sell" {
		side = Sell
	}
	return Order{
		ID:       fmt.Sprintf("%v", id),
		Symbol:   b.denormalizeSymbol(symbol),
		Side:     side,
		Price:    r,
		Quantity: a,
	}
}

func (b *BitkubExchange) GetSymbolInfo(ctx context.Context, symbol string) (SymbolInfo, error) {
	normSym := b.normalizeSymbol(symbol)
	resp, err := b.client.Get(b.BaseURL + "/api/v3/market/symbols")
	if err != nil {
		return SymbolInfo{}, err
	}
	defer resp.Body.Close()

	var data struct {
		Error  int `json:"error"`
		Result []struct {
			Symbol        string  `json:"symbol"`
			BaseAsset     string  `json:"base_asset"`
			QuoteAsset    string  `json:"quote_asset"`
			PriceScale    int     `json:"price_scale"`
			QuantityScale int     `json:"quantity_scale"`
			MinQuoteSize  float64 `json:"min_quote_size"`
			PriceStep     string  `json:"price_step"`
			QuantityStep  string  `json:"quantity_step"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return SymbolInfo{}, err
	}
	if data.Error != 0 {
		return SymbolInfo{}, fmt.Errorf("bitkub error code: %d", data.Error)
	}

	for _, s := range data.Result {
		if s.Symbol == normSym {
			ps, _ := strconv.ParseFloat(s.PriceStep, 64)
			qs, _ := strconv.ParseFloat(s.QuantityStep, 64)
			return SymbolInfo{
				Symbol:         b.denormalizeSymbol(s.Symbol),
				BaseAsset:      s.BaseAsset,
				QuoteAsset:     s.QuoteAsset,
				BasePrecision:  s.QuantityScale,
				QuotePrecision: s.PriceScale,
				MinQty:         qs,
				MinNotional:    s.MinQuoteSize,
				StepSize:       qs,
				TickSize:       ps,
			}, nil
		}
	}

	return SymbolInfo{}, fmt.Errorf("symbol not found: %s", normSym)
}
func (b *BitkubExchange) GetOrderBook(ctx context.Context, symbol string, limit int) (OrderBook, error) {
	normSym := b.normalizeSymbol(symbol)
	url := fmt.Sprintf("%s/api/v3/market/depth?sym=%s&lmt=%d", b.BaseURL, normSym, limit)
	resp, err := b.client.Get(url)
	if err != nil {
		return OrderBook{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return OrderBook{}, fmt.Errorf("bitkub depth failed: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return OrderBook{}, err
	}

	// Bitkub V3 can return either direct depth object or wrapped in an envelope
	var envelope struct {
		Error  int `json:"error"`
		Result struct {
			Asks [][]interface{} `json:"asks"`
			Bids [][]interface{} `json:"bids"`
		} `json:"result"`
	}

	var directData struct {
		Asks [][]interface{} `json:"asks"`
		Bids [][]interface{} `json:"bids"`
	}

	book := OrderBook{Symbol: symbol}
	var asks, bids [][]interface{}

	if err := json.Unmarshal(body, &envelope); err == nil && (len(envelope.Result.Asks) > 0 || len(envelope.Result.Bids) > 0) {
		asks = envelope.Result.Asks
		bids = envelope.Result.Bids
	} else if err := json.Unmarshal(body, &directData); err == nil {
		asks = directData.Asks
		bids = directData.Bids
	} else {
		return OrderBook{}, fmt.Errorf("failed to parse Bitkub depth response: %s", string(body))
	}

	for _, bid := range bids {
		if len(bid) >= 2 {
			p, _ := bid[0].(float64)
			q, _ := bid[1].(float64)
			book.Bids = append(book.Bids, DepthEntry{Price: p, Quantity: q})
		}
	}
	for _, ask := range asks {
		if len(ask) >= 2 {
			p, _ := ask[0].(float64)
			q, _ := ask[1].(float64)
			book.Asks = append(book.Asks, DepthEntry{Price: p, Quantity: q})
		}
	}
	return book, nil
}
