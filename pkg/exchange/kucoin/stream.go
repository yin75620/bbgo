package kucoin

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/c9s/bbgo/pkg/exchange/kucoin/kucoinapi"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
)

const readTimeout = 30 * time.Second

type WebsocketOp struct {
	Op   string      `json:"op"`
	Args interface{} `json:"args"`
}

type WebsocketLogin struct {
	Key        string `json:"apiKey"`
	Passphrase string `json:"passphrase"`
	Timestamp  string `json:"timestamp"`
	Sign       string `json:"sign"`
}

//go:generate callbackgen -type Stream -interface
type Stream struct {
	types.StandardStream

	client     *kucoinapi.RestClient
	conn       *websocket.Conn
	connLock   sync.Mutex
	connCtx    context.Context
	connCancel context.CancelFunc

	bullet     *kucoinapi.Bullet

	candleEventCallbacks         []func(e *kucoinapi.WebSocketCandle)
	orderBookL2EventCallbacks    []func(e *kucoinapi.WebSocketOrderBookL2)
	tickerEventCallbacks         []func(e *kucoinapi.WebSocketTicker)
	accountBalanceEventCallbacks []func(e *kucoinapi.WebSocketAccountBalance)
	privateOrderEventCallbacks   []func(e *kucoinapi.WebSocketPrivateOrder)
}

func NewStream(client *kucoinapi.RestClient) *Stream {
	stream := &Stream{
		client: client,
		StandardStream: types.StandardStream{
			ReconnectC: make(chan struct{}, 1),
		},
	}

	stream.OnConnect(stream.handleConnect)
	stream.OnCandleEvent(stream.handleCandleEvent)
	stream.OnOrderBookL2Event(stream.handleOrderBookL2Event)
	stream.OnTickerEvent(stream.handleTickerEvent)
	stream.OnPrivateOrderEvent(stream.handlePrivateOrderEvent)
	return stream
}

func (s *Stream) handleCandleEvent(e *kucoinapi.WebSocketCandle) {}

func (s *Stream) handleOrderBookL2Event(e *kucoinapi.WebSocketOrderBookL2) {}

func (s *Stream) handleTickerEvent(e *kucoinapi.WebSocketTicker) {}

func (s *Stream) handleAccountBalanceEvent(e *kucoinapi.WebSocketAccountBalance) {
	bm := types.BalanceMap{}
	bm[e.Currency] = types.Balance{
		Currency:  e.Currency,
		Available: e.Available,
		Locked:    e.Hold,
	}
	s.StandardStream.EmitBalanceUpdate(bm)
}

func (s *Stream) handlePrivateOrderEvent(e *kucoinapi.WebSocketPrivateOrder) {
	if e.Type == "match" {
		s.StandardStream.EmitTradeUpdate(types.Trade{
			OrderID:       hashStringID(e.OrderId),
			ID:            hashStringID(e.TradeId),
			Exchange:      types.ExchangeKucoin,
			Price:         e.MatchPrice.Float64(),
			Quantity:      e.MatchSize.Float64(),
			QuoteQuantity: e.MatchPrice.Float64() * e.MatchSize.Float64(),
			Symbol:        toGlobalSymbol(e.Symbol),
			Side:          toGlobalSide(e.Side),
			IsBuyer:       e.Side == "buy",
			IsMaker:       e.Liquidity == "maker",
			Time:          types.Time(e.Ts.Time()),
			Fee:           0,  // not supported
			FeeCurrency:   "", // not supported
		})
	}

	switch e.Type {
	case "open", "match", "filled":
		status := types.OrderStatusNew
		if e.Status == "done" {
			status = types.OrderStatusFilled
		} else if e.FilledSize > 0 {
			status = types.OrderStatusPartiallyFilled
		}

		s.StandardStream.EmitOrderUpdate(types.Order{
			SubmitOrder: types.SubmitOrder{
				Symbol:   toGlobalSymbol(e.Symbol),
				Side:     toGlobalSide(e.Side),
				Type:     toGlobalOrderType(e.OrderType),
				Quantity: e.Size.Float64(),
				Price:    e.Price.Float64(),
			},
			Exchange:         types.ExchangeKucoin,
			OrderID:          hashStringID(e.OrderId),
			Status:           status,
			ExecutedQuantity: e.FilledSize.Float64(),
			IsWorking:        true,
			CreationTime:     types.Time(e.OrderTime.Time()),
			UpdateTime:       types.Time(e.Ts.Time()),
		})

	default:
		log.Warnf("unhandled private order type: %s, payload: %+v", e.Type, e)

	}
}

func (s *Stream) handleConnect() {
	if s.PublicOnly {
		if err := s.sendSubscriptions(); err != nil {
			log.WithError(err).Errorf("subscription error")
			return
		}
	} else {
		id := time.Now().UnixMilli()
		cmds := []kucoinapi.WebSocketCommand{
			{
				Id:             id,
				Type:           kucoinapi.WebSocketMessageTypeSubscribe,
				Topic:          "/spotMarket/tradeOrders",
				PrivateChannel: true,
				Response:       true,
			},
			{
				Id:             id + 1,
				Type:           kucoinapi.WebSocketMessageTypeSubscribe,
				Topic:          "/account/balance",
				PrivateChannel: true,
				Response:       true,
			},
		}
		for _, cmd := range cmds {
			if err := s.conn.WriteJSON(cmd); err != nil {
				log.WithError(err).Errorf("private subscribe write error, cmd: %+v", cmd)
			}
		}
	}
}


func (s *Stream) Close() error {
	conn := s.Conn()
	return conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
}

func (s *Stream) Connect(ctx context.Context) error {
	err := s.connect(ctx)
	if err != nil {
		return err
	}

	// start one re-connector goroutine with the base context
	go s.Reconnector(ctx)

	s.EmitStart()
	return nil
}

func (s *Stream) Reconnector(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return

		case <-s.ReconnectC:
			log.Warnf("received reconnect signal, reconnecting...")
			time.Sleep(3 * time.Second)

			if err := s.connect(ctx); err != nil {
				log.WithError(err).Errorf("connect error, try to reconnect again...")
				s.Reconnect()
			}
		}
	}
}

func (s *Stream) sendSubscriptions() error {
	cmds, err := convertSubscriptions(s.Subscriptions)
	if err != nil {
		return errors.Wrapf(err, "subscription convert error, subscriptions: %+v", s.Subscriptions)
	}

	for _, cmd := range cmds {
		if err := s.conn.WriteJSON(cmd); err != nil {
			return errors.Wrapf(err, "subscribe write error, cmd: %+v", cmd)
		}
	}

	return nil
}

// getEndpoint use the PublicOnly flag to check whether we should allocate a public bullet or private bullet
func (s *Stream) getEndpoint() (string, error) {
	var bullet *kucoinapi.Bullet
	var err error
	if s.PublicOnly {
		bullet, err = s.client.BulletService.NewGetPublicBulletRequest().Do(nil)
	} else {
		bullet, err = s.client.BulletService.NewGetPrivateBulletRequest().Do(nil)
	}

	if err != nil {
		return "", err
	}

	url, err := bullet.URL()
	if err != nil {
		return "", err
	}

	s.bullet = bullet

	log.Infof("bullet: %+v", bullet)
	return url.String(), nil
}

func (s *Stream) connect(ctx context.Context) error {
	url, err := s.getEndpoint()
	if err != nil {
		return err
	}

	conn, err := s.StandardStream.Dial(url)
	if err != nil {
		return err
	}

	log.Infof("websocket connected: %s", url)

	// should only start one connection one time, so we lock the mutex
	s.connLock.Lock()

	// ensure the previous context is cancelled
	if s.connCancel != nil {
		s.connCancel()
	}

	// create a new context
	s.connCtx, s.connCancel = context.WithCancel(ctx)

	// pingTimeout := s.bullet.PingTimeout()
	conn.SetReadDeadline(time.Now().Add(readTimeout))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(readTimeout))
		return nil
	})

	s.conn = conn
	s.connLock.Unlock()

	s.EmitConnect()

	go s.read(s.connCtx)
	go ping(s.connCtx, s, s.bullet.PingInterval())
	return nil
}

func (s *Stream) read(ctx context.Context) {
	defer func() {
		if s.connCancel != nil {
			s.connCancel()
		}
		s.EmitDisconnect()
	}()

	for {
		select {

		case <-ctx.Done():
			return

		default:
			conn := s.Conn()

			if err := conn.SetReadDeadline(time.Now().Add(readTimeout)); err != nil {
				log.WithError(err).Errorf("set read deadline error: %s", err.Error())
			}

			mt, message, err := conn.ReadMessage()
			if err != nil {
				// if it's a network timeout error, we should re-connect
				switch err := err.(type) {

				// if it's a websocket related error
				case *websocket.CloseError:
					if err.Code == websocket.CloseNormalClosure {
						return
					}

					// for unexpected close error, we should re-connect
					// emit reconnect to start a new connection
					s.Reconnect()
					return

				case net.Error:
					log.WithError(err).Error("network error")
					s.Reconnect()
					return

				default:
					log.WithError(err).Error("unexpected connection error")
					s.Reconnect()
					return
				}
			}

			// skip non-text messages
			if mt != websocket.TextMessage {
				continue
			}

			// used for debugging
			// fmt.Println(string(message))

			e, err := parseWebsocketPayload(message)
			if err != nil {
				log.WithError(err).Error("message parse error")
				continue
			}

			// remove bytes, so we won't print them
			e.Data = nil
			log.Infof("event: %+v", e)

			if e != nil && e.Object != nil {
				s.dispatchEvent(e)
			}
		}
	}
}

func (s *Stream) dispatchEvent(e *kucoinapi.WebSocketEvent) {
	switch et := e.Object.(type) {

	case *kucoinapi.WebSocketTicker:
		s.EmitTickerEvent(et)

	case *kucoinapi.WebSocketOrderBookL2:
		s.EmitOrderBookL2Event(et)

	case *kucoinapi.WebSocketCandle:
		s.EmitCandleEvent(et)

	case *kucoinapi.WebSocketAccountBalance:
		s.EmitAccountBalanceEvent(et)

	case *kucoinapi.WebSocketPrivateOrder:
		s.EmitPrivateOrderEvent(et)

	default:
		log.Warnf("unhandled event: %+v", et)

	}
}

func (s *Stream) Conn() *websocket.Conn {
	s.connLock.Lock()
	conn := s.conn
	s.connLock.Unlock()
	return conn
}

type WebSocketConnector interface {
	Conn() *websocket.Conn
	Reconnect()
}

func ping(ctx context.Context, w WebSocketConnector, interval time.Duration) {
	log.Infof("starting ping worker with interval %s", interval)

	pingTicker := time.NewTicker(interval)
	defer pingTicker.Stop()

	for {
		select {

		case <-ctx.Done():
			log.Debug("ping worker stopped")
			return

		case <-pingTicker.C:
			conn := w.Conn()

			if err := conn.WriteJSON(kucoinapi.WebSocketCommand{
				Id:   time.Now().UnixMilli(),
				Type: "ping",
			}); err != nil {
				log.WithError(err).Error("websocket ping error", err)
				w.Reconnect()
			}

			if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(3*time.Second)); err != nil {
				log.WithError(err).Error("ping error", err)
				w.Reconnect()
			}
		}
	}
}