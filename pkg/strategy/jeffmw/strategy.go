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

	//setting
	LimitLowerHighTimes int              `json:"limitLowerHightTimes"`
	InitialUsd          fixedpoint.Value `json:"initialUsd"` //if 0 => 100
	Leverage            fixedpoint.Value `json:"leverage"`   // if 0 => 1
	WinLeftCount        int              `json:"winLeftCount"`
	WinRightCount       int              `json:"winRightCount"`
	AllowRightUpPercent float64          `json:"allowRightUpPercent"` //0.012就表示右邊低點往上1.012倍後會比左邊低點高
	IsCompoundOrder     bool             `json:"isCompoundOrder"`

	//
	IncreaseVoScale      fixedpoint.Value `json:"increaseVolScale"`
	IncreasePriceScale   fixedpoint.Value `json:"increasePriceScale"`
	AmplificationPercent fixedpoint.Value `json:"amplificationPercent"` //大於
	ChangeRatio          fixedpoint.Value `json:"changeRatio"`          //大於
	UpperPowerRatio      fixedpoint.Value `json:"upperPowerRatio"`      //大於
	UpperShadowRatio     fixedpoint.Value `json:"upperShadowRatio"`     //小於

	OverAmplificationPercent fixedpoint.Value `json:"overAmplificationPercent"` //小於

	// start info
	configUsdValue fixedpoint.Value

	positionKline     types.KLine
	lowerHighTimes    int
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
	jwmchart := standardIndicatorSet.JWMChart(iw, s.WinLeftCount, s.WinRightCount, s.AllowRightUpPercent)

	var vmaIw = types.IntervalWindow{Interval: s.MovingAverage.Interval, Window: s.VmaWindow}
	vma := standardIndicatorSet.VMA(vmaIw)

	var smaIw = types.IntervalWindow{Interval: s.MovingAverage.Interval, Window: s.VmaWindow}
	sma := standardIndicatorSet.SMA(smaIw)

	var highXmaIw = types.IntervalWindow{Interval: s.MovingAverage.Interval, Window: s.VmaWindow}
	highXma := standardIndicatorSet.XMA(highXmaIw, "high", func(k types.KLine) float64 {
		return k.High.Float64()
	})
	fmt.Println(highXma)

	var spoorVolXmaIw = types.IntervalWindow{Interval: s.MovingAverage.Interval, Window: s.VmaWindow}
	spoorVol := standardIndicatorSet.XMA(spoorVolXmaIw, "spoorVol", func(k types.KLine) float64 {
		//上影線下影線各算兩次，實Ｋ算一次，價格變化總和/最低價，算出波動率，成交量/波動率 /100，可知一個1%波動率，要多少成交量
		//越大越穩
		onePercentSpoorRatio := k.GetOnePercentSpoorVol()
		return onePercentSpoorRatio.Float64()
	})
	fmt.Println(spoorVol)

	s.positionKline = types.KLine{}
	usdtBalance, _ := session.Account.Balance("USDT")
	s.configUsdValue = usdtBalance.Total()

	// prepare function to sell position
	SellFunc := func(kline types.KLine) {
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

	// skip k-lines from other symbols
	session.MarketDataStream.OnKLineClosed(types.KLineWith(s.Symbol, s.MovingAverage.Interval, func(kline types.KLine) {

		//fmt.Println(kline.StartTime)

		last := jwmchart.Last()
		if s.HasPosition() {

			//止損策略
			// if kline.Close < s.positionKline.Low-s.positionKline.GetChange()*2 {
			// 	//SellFunc(kline)
			// 	return
			// }

			if last.HighLoseLeftIndex == 1 {
				s.lowerHighTimes += 1
			}

			if s.lowerHighTimes > s.LimitLowerHighTimes {
				SellFunc(kline)
				return
			}
		}

		// Buy Check

		// 軌跡波動量超越均量特定比例
		//spoorRate := kline.GetOnePercentSpoorVol().Div(fixedpoint.NewFromFloat(spoorVol.Last()))
		//if spoorRate.Sub(fixedpoint.NewFromFloat(1.08)) > fixedpoint.Zero {
		//	return
		//}

		//成交量的/超越指定比例
		if kline.Volume.Div(fixedpoint.NewFromFloat(vma.Index(1))).Sub(s.IncreaseVoScale) < fixedpoint.Zero {
			return
		}

		//
		if kline.Close.Div(fixedpoint.NewFromFloat(sma.Index(1))).Sub(s.IncreasePriceScale) < fixedpoint.Zero {
			return
		}

		//K線本身品質檢查
		if kline.GetAmplification().Sub(s.AmplificationPercent) < fixedpoint.Zero {
			//波動超過X
			return
		}

		if kline.GetAmplification().Sub(s.OverAmplificationPercent) > fixedpoint.Zero {
			//波動太超過，就剔除
			return
		}

		// 實K要超過特定比例
		if kline.GetThickness().Sub(s.ChangeRatio) < fixedpoint.Zero {
			return
		}

		// 向上力道要超過特定比例
		if kline.GetUpperPowerRatio().Sub(s.UpperPowerRatio) < fixedpoint.Zero {
			return
		}

		//上影線要小於特定比例
		if fixedpoint.One.Sub(kline.GetUpperShadowRatio()).Sub(s.UpperShadowRatio) < fixedpoint.Zero {
			return
		}

		if last.IsKillTopKline { // canBuy

			s.lowerHighTimes = 0

			if !s.HasPosition() {

				// order
				orderUSD := s.InitialUsd

				// money check
				usdtBalance, _ := session.Account.Balance("USDT")
				revenue := usdtBalance.Total().Sub(s.configUsdValue)
				totalAvalableUSD := orderUSD.Add(revenue)

				if totalAvalableUSD < 0 {
					//money not enough
					return
				}

				if s.IsCompoundOrder {
					orderUSD = totalAvalableUSD
				}

				//計算要下單的數量
				orderUSD = orderUSD.Mul(s.Leverage)

				quantity := orderUSD.Div(kline.Close) //fixedpoint.NewFromFloat(0.01)

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
				log.Infoln("already has position")
			}
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
