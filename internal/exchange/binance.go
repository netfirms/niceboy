package exchange

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/adshao/go-binance/v2"
)

type BinanceExchange struct {
	client *binance.Client
}

func NewBinanceExchange(apiKey, secretKey string) *BinanceExchange {
	c := binance.NewClient(apiKey, secretKey)
	return &BinanceExchange{
		client: c,
	}
}

func (b *BinanceExchange) GetName() string {
	return "binance"
}

func (b *BinanceExchange) GetPrice(ctx context.Context, symbol string) (float64, error) {
	prices, err := b.client.NewListPricesService().Symbol(symbol).Do(ctx)
	if err != nil {
		return 0, err
	}
	if len(prices) == 0 {
		return 0, fmt.Errorf("no price found for symbol: %s", symbol)
	}
	return strconv.ParseFloat(prices[0].Price, 64)
}

func (b *BinanceExchange) SubscribePrice(ctx context.Context, symbol string, ch chan<- MarketData) error {
	wsHandler := func(event *binance.WsBookTickerEvent) {
		price, _ := strconv.ParseFloat(event.BestBidPrice, 64)
		if price == 0 {
			price, _ = strconv.ParseFloat(event.BestAskPrice, 64)
		}
		
		select {
		case <-ctx.Done():
			return
		case ch <- MarketData{
			Symbol: symbol,
			Price:  price,
			Time:   time.Now().UnixNano() / 1e6,
		}:
		}
	}
	
	go func() {
		backoff := time.Second
		for {
			select {
			case <-ctx.Done():
				return
			default:
				errHandler := func(err error) {
					// Connection lost
				}

				doneC, stopC, err := binance.WsBookTickerServe(symbol, wsHandler, errHandler)
				if err != nil {
					time.Sleep(backoff)
					if backoff < 30*time.Second {
						backoff *= 2
					}
					continue
				}

				backoff = time.Second // Reset on success

				select {
				case <-ctx.Done():
					stopC <- struct{}{}
					<-doneC
					return
				case <-doneC:
					// Stream closed externally or errored, loop to reconnect
					time.Sleep(backoff)
					if backoff < 30*time.Second {
						backoff *= 2
					}
				}
			}
		}
	}()

	return nil
}

func (b *BinanceExchange) GetBalances(ctx context.Context) (map[string]float64, error) {
	acc, err := b.client.NewGetAccountService().Do(ctx)
	if err != nil {
		return nil, err
	}
	balances := make(map[string]float64)
	for _, bal := range acc.Balances {
		var free float64
		fmt.Sscanf(bal.Free, "%f", &free)
		var locked float64
		fmt.Sscanf(bal.Locked, "%f", &locked)
		total := free + locked
		if total > 0 {
			balances[bal.Asset] = total
		}
	}
	return balances, nil
}

func (b *BinanceExchange) GetOpenOrders(ctx context.Context, symbol string) ([]Order, error) {
	orders, err := b.client.NewListOpenOrdersService().Symbol(symbol).Do(ctx)
	if err != nil {
		return nil, err
	}
	
	var result []Order
	for _, o := range orders {
		var p, q float64
		fmt.Sscanf(o.Price, "%f", &p)
		fmt.Sscanf(o.OrigQuantity, "%f", &q)
		
		side := Buy
		if o.Side == binance.SideTypeSell {
			side = Sell
		}
		
		result = append(result, Order{
			ID:       fmt.Sprintf("%d", o.OrderID),
			Symbol:   o.Symbol,
			Side:     side,
			Price:    p,
			Quantity: q,
		})
	}
	return result, nil
}

func (b *BinanceExchange) ExecuteOrder(ctx context.Context, symbol string, side OrderSide, orderType OrderType, quantity float64, price float64) error {
	sideType := binance.SideTypeBuy
	if side == Sell {
		sideType = binance.SideTypeSell
	}

	oType := binance.OrderTypeMarket
	if orderType == Limit {
		oType = binance.OrderTypeLimit
	}

	srv := b.client.NewCreateOrderService().
		Symbol(symbol).
		Side(sideType).
		Type(oType).
		Quantity(strconv.FormatFloat(quantity, 'f', -1, 64))

	if orderType == Limit {
		srv = srv.Price(strconv.FormatFloat(price, 'f', -1, 64))
	}

	_, err := srv.Do(ctx)
	return err
}
