package jeffmw

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "jeffmw"

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
	VmaWindow     int                  `json:"vmaWindow"`

	InitialUsd fixedpoint.Value `json:"initialUsd"` //if 0 => 100
	Leverage   fixedpoint.Value `json:"leverage"`   // if 0 => 1

	// start info
	configUsdValue fixedpoint.Value

	MChartTactic MChartTactic `json:"mChartTactic"`
	WChartTactic WChartTactic `json:"wChartTactic"`
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

	if s.VmaWindow == 0 {
		s.VmaWindow = 99
	}

	if s.InitialUsd == 0 {
		s.InitialUsd = fixedpoint.NewFromFloat(100.0)
	}
	if s.Leverage == 0 {
		s.Leverage = 1
	}

	standardIndicatorSet := session.StandardIndicatorSet(s.Symbol)
	if standardIndicatorSet == nil {
		return fmt.Errorf("standardIndicatorSet is nil, symbol %s", s.Symbol)
	}

	var iw = types.IntervalWindow{Interval: s.MovingAverage.Interval, Window: s.MovingAverage.Window}
	jwmchart := standardIndicatorSet.JWMChart(iw)

	var vmaIw = types.IntervalWindow{Interval: s.MovingAverage.Interval, Window: s.VmaWindow}
	vma := standardIndicatorSet.VMA(vmaIw)

	var smaIw = types.IntervalWindow{Interval: s.MovingAverage.Interval, Window: s.VmaWindow}
	sma := standardIndicatorSet.SMA(smaIw)

	// var LowXmaIw = types.IntervalWindow{Interval: s.MovingAverage.Interval, Window: 1}
	// lowXMA := standardIndicatorSet.XMA(LowXmaIw, "lowXmaIw", func(k types.KLine) float64 {
	// 	return k.Low.Float64()
	// })
	// fmt.Println(lowXMA)

	// var spoorVolXmaIw = types.IntervalWindow{Interval: s.MovingAverage.Interval, Window: s.VmaWindow}
	// spoorVol := standardIndicatorSet.XMA(spoorVolXmaIw, "spoorVol", func(k types.KLine) float64 {
	// 	//上影線下影線各算兩次，實Ｋ算一次，價格變化總和/最低價，算出波動率，成交量/波動率 /100，可知一個1%波動率，要多少成交量
	// 	//越大越穩
	// 	onePercentSpoorRatio := k.GetOnePercentSpoorVol()
	// 	return onePercentSpoorRatio.Float64()
	// })
	// fmt.Println(spoorVol)

	usdtBalance, _ := session.Account.Balance("USDT")
	s.configUsdValue = usdtBalance.Total()

	repeater := Repeater{
		jwmchart:      jwmchart,
		vma:           vma,
		sma:           sma,
		ctx:           ctx,
		orderExecutor: orderExecutor,
		session:       session,
		market:        market}

	s.WChartTactic.Init(s)

	// skip k-lines from other symbols
	session.MarketDataStream.OnKLineClosed(types.KLineWith(s.Symbol, s.MovingAverage.Interval, func(kline types.KLine) {

		s.WChartTactic.OnKLineClosed(kline, repeater)

	}))

	return nil
}

type Repeater struct {
	jwmchart      *indicator.JWMChart
	vma           *indicator.VMA
	sma           *indicator.SMA
	ctx           context.Context
	orderExecutor bbgo.OrderExecutor
	session       *bbgo.ExchangeSession
	market        types.Market
}

// InstanceID returns the instance identifier from the current grid configuration parameters
func (s *Strategy) InstanceID() string {
	return fmt.Sprintf("%s-%s", ID, s.Symbol)
}
