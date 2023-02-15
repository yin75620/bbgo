package jeff1

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "jeffm"

var log = logrus.WithField("strategy", ID)
var cpiTimeUnix = time.Time{}.UnixMilli()

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type State struct {
	Position *types.Position `json:"position,omitempty"`
}

type Strategy struct {
	Symbol        string               `json:"symbol"`
	MovingAverage types.IntervalWindow `json:"movingAverage"`

	CheckVolumeMultiple float64 `json:"checkVolumeMultiple"`
	UpperPowerStand     float64 `json:"upperPowerStand"`
	LowerPowerStand     float64 `json:"lowerPowerStand"`
	VmaUpperRatio       float64 `json:"vmaUpperRatio"`
	//State *State `persistence:"state"`

	//ProfitStats *types.ProfitStats `persistence:"profit_stats"`

	// orderStore is used to store all the created orders, so that we can filter the trades.
	//orderStore     *bbgo.OrderStore
	//tradeCollector *bbgo.TradeCollector
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

	standardIndicatorSet := session.StandardIndicatorSet(s.Symbol)
	if standardIndicatorSet == nil {
		return fmt.Errorf("standardIndicatorSet is nil, symbol %s", s.Symbol)
	}

	var iw = types.IntervalWindow{Interval: s.MovingAverage.Interval, Window: s.MovingAverage.Window}
	vma := standardIndicatorSet.VMA(iw)

	//持倉狀態
	/*
		s.tradeCollector = bbgo.NewTradeCollector(s.Symbol, s.State.Position, s.orderStore)

		s.tradeCollector.OnTrade(func(trade types.Trade, profit, netProfit fixedpoint.Value) {
			bbgo.Notify(trade)
			s.ProfitStats.AddTrade(trade)
		})

		s.tradeCollector.OnPositionUpdate(func(position *types.Position) {
			bbgo.Notify(position)
		})
		s.tradeCollector.BindStream(session.UserDataStream)
	*/

	session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
		// skip k-lines from other symbols
		if kline.Symbol != s.Symbol {
			return
		}

		if kline.Interval != s.MovingAverage.Interval {
			return
		}

		log.Infof("%s", kline.String())

		//成交量大於均量N倍 N=3
		checkVolumeMultiple := s.CheckVolumeMultiple
		upperPowerStand := s.UpperPowerStand
		lowerPowerStand := s.LowerPowerStand
		vmaUpperRatio := s.VmaUpperRatio
		orderUSD := fixedpoint.NewFromFloat(100.0)
		quantity := orderUSD.Div(kline.Close) //fixedpoint.NewFromFloat(0.01)

		//LowerPowerRatio>ML 就賣
		lp := kline.GetLowerPowerRatio().Float64()
		if lp > lowerPowerStand {
			_, err := orderExecutor.SubmitOrders(ctx, types.SubmitOrder{
				Symbol:   kline.Symbol,
				Market:   market,
				Side:     types.SideTypeSell,
				Type:     types.OrderTypeMarket,
				Quantity: quantity,
			})
			if err != nil {
				log.WithError(err).Error("subit sell order error")
			}
		}

		if kline.Volume.Float64()/vma.Last() < checkVolumeMultiple {
			return
		}

		if vma.Last()/vma.Index(1) < vmaUpperRatio {
			return
		}

		//UpperPowerRatio>M 就買
		up := kline.GetUpperPowerRatio().Float64()
		if up > upperPowerStand {

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
		}

	})

	return nil
}

// InstanceID returns the instance identifier from the current grid configuration parameters
func (s *Strategy) InstanceID() string {
	return fmt.Sprintf("%s-%s-%f-%f-%f-%f", ID, s.Symbol, s.CheckVolumeMultiple, s.UpperPowerStand, s.LowerPowerStand, s.VmaUpperRatio)
}
