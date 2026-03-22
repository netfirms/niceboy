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

func (b *BitkubExchange) GetName() string {
	return "bitkub"
}

func (b *BitkubExchange) GetPrice(ctx context.Context, symbol string) (float64, error) {
	url := fmt.Sprintf("%s/api/market/ticker?sym=%s", b.BaseURL, symbol)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, err
	}

	resp, err := b.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var data struct {
		Result map[string]struct {
			Last float64 `json:"last"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return 0, err
	}

	ticker, ok := data.Result[symbol]
	if !ok {
		return 0, fmt.Errorf("no ticker data for symbol: %s", symbol)
	}

	return ticker.Last, nil
}

func (b *BitkubExchange) SubscribePrice(ctx context.Context, symbol string, ch chan<- MarketData) error {
	streamName := strings.ToLower(fmt.Sprintf("market.ticker.%s", symbol))
	wsURL := fmt.Sprintf("%s/%s", b.BaseWSURL, streamName)

	c, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		return fmt.Errorf("bitkub ws dial error: %v", err)
	}

	go func() {
		defer c.Close()
		for {
			select {
			case <-ctx.Done():
				return
			default:
				_, message, err := c.ReadMessage()
				if err != nil {
					// In a real app we'd attempt to reconnect, but here we just return
					return
				}

				var data struct {
					Last float64 `json:"last"`
				}
				if err := json.Unmarshal(message, &data); err == nil && data.Last > 0 {
					ch <- MarketData{
						Symbol: symbol,
						Price:  data.Last,
						Time:   time.Now().UnixNano() / 1e6, // current time ms
					}
				}
			}
		}
	}()
	return nil
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

	payload := map[string]interface{}{
		"sym":  symbol,
		"amt":  quantity, // Bitkub 'amt' definition depends on buy/sell but we pass it generically.
		"rat":  price,
		"type": typ,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	ts := time.Now().UnixNano() / 1e6
	sigStr := fmt.Sprintf("%dPOST%s%s", ts, endpoint, string(payloadBytes))

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
	
	ts := time.Now().UnixNano() / 1e6
	sigStr := fmt.Sprintf("%dPOST%s", ts, endpoint)

	mac := hmac.New(sha256.New, []byte(b.secret))
	mac.Write([]byte(sigStr))
	sig := hex.EncodeToString(mac.Sum(nil))

	req, err := http.NewRequestWithContext(ctx, "POST", b.BaseURL+endpoint, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
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
	query := "sym=" + strings.ToUpper(symbol)

	ts := time.Now().UnixNano() / 1e6
	sigStr := fmt.Sprintf("%dGET%s?%s", ts, endpoint, query)

	mac := hmac.New(sha256.New, []byte(b.secret))
	mac.Write([]byte(sigStr))
	sig := hex.EncodeToString(mac.Sum(nil))

	req, err := http.NewRequestWithContext(ctx, "GET", b.BaseURL+endpoint+"?"+query, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
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

	// But according to docs, rate and amount might be strings,
	// wait, let's parse using json.Unmarshal with interface{} or json.RawMessage if we want to be safe,
	// or decode as string? The V3 docs examples show them as float or string randomly.
	// We'll decode using string and parse float if needed. I will use struct with string type.
	
	body, _ := io.ReadAll(resp.Body)
	var strData struct {
		Error  int `json:"error"`
		Result []struct {
			ID     interface{} `json:"id"`
			Side   string      `json:"side"`
			Rate   interface{} `json:"rate"`
			Amount interface{} `json:"amount"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &strData); err != nil {
		return nil, err
	}
	if strData.Error != 0 {
		return nil, fmt.Errorf("bitkub error code: %d", strData.Error)
	}

	var res []Order
	for _, o := range strData.Result {
		var rate, amount float64
		// Parse rate
		switch v := o.Rate.(type) {
		case float64:
			rate = v
		case string:
			fmt.Sscanf(v, "%f", &rate)
		}
		// Parse amount
		switch v := o.Amount.(type) {
		case float64:
			amount = v
		case string:
			fmt.Sscanf(v, "%f", &amount)
		}

		side := Buy
		if strings.ToLower(o.Side) == "sell" {
			side = Sell
		}
		res = append(res, Order{
			ID:       fmt.Sprintf("%v", o.ID),
			Symbol:   symbol,
			Side:     side,
			Price:    rate,
			Quantity: amount,
		})
	}
	return res, nil
}
