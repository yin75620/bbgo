package oneTrade

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "oneTrade"

var log = logrus.WithField("strategy", ID)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type State struct {
	Position *types.Position `json:"position,omitempty"`
}

type Strategy struct {
	Symbol        string               `json:"symbol"`
	MovingAverage types.IntervalWindow `json:"movingAverage"`

	positionKline     types.KLine
	lastOrderQuantity fixedpoint.Value
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.MovingAverage.Interval})
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {

	market, ok := session.Market(s.Symbol)
	if !ok {
		return fmt.Errorf("market %s is not defined", s.Symbol)
	}

	if s.MovingAverage.Interval == "" {
		s.MovingAverage.Interval = types.Interval1m
	}

	if s.MovingAverage.Window == 0 {
		s.MovingAverage.Window = 99
	}

	s.positionKline = types.KLine{}

	// skip k-lines from other symbols
	session.MarketDataStream.OnKLineClosed(types.KLineWith(s.Symbol, s.MovingAverage.Interval, func(kline types.KLine) {

		if !s.HasPosition() {
			// order
			orderUSD := fixedpoint.NewFromFloat(100)
			quantity := orderUSD.Div(kline.Close)

			//執行購買
			_, err := orderExecutor.SubmitOrders(ctx, types.SubmitOrder{
				Symbol:   kline.Symbol,
				Market:   market,
				Side:     types.SideTypeBuy,
				Type:     types.OrderTypeMarket,
				Quantity: quantity,
			})
			if err != nil {
				log.WithError(err).Error("subit buy order error")
			}
			s.positionKline = kline
			s.lastOrderQuantity = quantity
		} else {

			_, err := orderExecutor.SubmitOrders(ctx, types.SubmitOrder{
				Symbol:   kline.Symbol,
				Market:   market,
				Side:     types.SideTypeSell,
				Type:     types.OrderTypeMarket,
				Quantity: s.lastOrderQuantity,
			})
			if err != nil {
				log.WithError(err).Error("subit sell order error")
			}
			s.positionKline = types.KLine{}
			s.lastOrderQuantity = 0
		}

	}))

	return nil
}

// InstanceID returns the instance identifier from the current grid configuration parameters
func (s *Strategy) InstanceID() string {
	return fmt.Sprintf("%s-%s", ID, s.Symbol)
}

func (s *Strategy) HasPosition() bool {
	return s.positionKline.Volume != fixedpoint.Zero
}
