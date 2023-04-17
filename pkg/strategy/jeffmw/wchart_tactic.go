package jeffmw

import (
	"fmt"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/sirupsen/logrus"
)

type WChartTactic struct {
	//configUsdValue fixedpoint.Value

	//setting
	LimitLowerHighTimes int              `json:"limitLowerHightTimes"`
	initialUsd          fixedpoint.Value //`json:"initialUsd"` //if 0 => 100
	leverage            fixedpoint.Value //`json:"leverage"`   // if 0 => 1
	WinLeftCount        int              `json:"winLeftCount"`
	WinRightCount       int              `json:"winRightCount"`
	SumWidthMin         int              `json:"sumWidthMin"`
	WinMaxMul           int              `json:"winMaxMul"`
	AllowRightUpPercent float64          `json:"allowRightUpPercent"` //0.012就表示右邊低點往上1.012倍後會比左邊低點高
	IsCompoundOrder     bool             `json:"isCompoundOrder"`

	//
	IncreaseVolScale     fixedpoint.Value `json:"increaseVolScale"`
	IncreasePriceScale   fixedpoint.Value `json:"increasePriceScale"`
	GainVolPreDayScale   fixedpoint.Value `json:"gainVolPreDayScale"`
	AmplificationPercent fixedpoint.Value `json:"amplificationPercent"` //大於
	ChangeRatio          fixedpoint.Value `json:"changeRatio"`          //大於
	UpperPowerRatio      fixedpoint.Value `json:"upperPowerRatio"`      //大於
	UpperShadowRatio     fixedpoint.Value `json:"upperShadowRatio"`     //小於

	OverAmplificationPercent fixedpoint.Value `json:"overAmplificationPercent"` //小於

	ForwardWidth     int `json:"ForwardWidth"`
	LoseLeftIndexMin int `json:"LoseLeftIndexMin"`

	// start info
	configUsdValue    fixedpoint.Value
	positionKline     types.KLine
	lowerHighTimes    int
	lastOrderQuantity fixedpoint.Value
	klineLow          fixedpoint.Value

	repeater *Repeater
}

func NewWChartTactic() *WChartTactic {
	return &WChartTactic{}
}

func (wct *WChartTactic) Init(s *Strategy, repeater *Repeater) {
	if wct.WinMaxMul == 0 {
		wct.WinMaxMul = 1000
	}

	wct.positionKline = types.KLine{}
	wct.configUsdValue = s.configUsdValue

	wct.initialUsd = s.InitialUsd
	wct.leverage = s.Leverage
	wct.repeater = repeater

}

func (s *WChartTactic) OnKLineClosed(kline types.KLine) {
	repeater := s.repeater
	jwmchart := repeater.jwmchart
	jwchart := repeater.jwchart
	vma := repeater.vma
	sma := repeater.sma
	session := repeater.session
	ctx := repeater.ctx
	orderExecutor := repeater.orderExecutor
	market := repeater.market

	last := jwchart.Last()

	// prepare function to sell position
	SellFunc := func(kline types.KLine) {
		_, err := orderExecutor.SubmitOrders(ctx, types.SubmitOrder{
			Symbol:           kline.Symbol,
			Market:           market,
			Side:             types.SideTypeSell,
			Type:             types.OrderTypeMarket,
			Quantity:         s.lastOrderQuantity,
			MarginSideEffect: types.SideEffectTypeAutoRepay,
		})
		if err != nil {
			log.WithError(err).Error("subit sell order error")
		}
		s.positionKline = types.KLine{}
		s.lastOrderQuantity = fixedpoint.Zero
		s.klineLow = fixedpoint.Zero
	}

	if s.HasPosition() { //prepare sell

		//止損策略
		// if kline.Close < s.positionKline.Low-s.positionKline.GetChange()*2 {
		// 	//SellFunc(kline)
		// 	return
		// }

		if last.LoseLeftIndex == 1 {
			s.lowerHighTimes += 1
		}

		// if kline.Close > s.positionKline.High && kline.Volume < s.positionKline.Volume {
		// 	SellFunc(kline)
		// 	return
		// }

		if s.lowerHighTimes > s.LimitLowerHighTimes ||
			kline.Close.Sub(s.klineLow) < fixedpoint.Zero {
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

	//成交量的/超越均量指定比例
	if kline.Volume.Div(fixedpoint.NewFromFloat(vma.Index(1))).Sub(s.IncreaseVolScale) < fixedpoint.Zero {
		logrus.Debug("未達成-成交量的/超越均量指定比例")
		return
	}

	//成交量比前一根高出指定比例
	if kline.Volume.Div(jwmchart.Index(1).K.Volume).Sub(s.GainVolPreDayScale) < fixedpoint.Zero {
		logrus.Debug("未達成-成交量比前一根高出指定比例")
		return
	}

	//找到輸掉的那一根Ｋ線，再往前N跟，如果有出現尖頭，也不交易
	if s.LoseLeftIndexMin != 0 {
		leftSideKinfos := jwmchart.IndexWidth(last.LoseLeftIndex, s.ForwardWidth)
		topKinfos := leftSideKinfos.GetWLoseLeftIndexLargerThan(s.LoseLeftIndexMin)

		if len(topKinfos) != 0 {
			logrus.Debug("未達成-找到輸掉的那一根Ｋ線")
			return
		}
	}

	//成交量過大於均量N倍剔除
	//maxRatio := 14.0 // default as 9999
	//if kline.Volume.Div(fixedpoint.NewFromFloat(vma.Index(1))).Sub(fixedpoint.NewFromFloat(maxRatio)) > fixedpoint.Zero {
	//	return
	//}

	// if kline.Volume.Div(fixedpoint.NewFromFloat(vma.Index(1))).Sub(fixedpoint.NewFromFloat(4.3)) < fixedpoint.Zero {
	// 	return
	// }

	//
	if kline.Close.Div(fixedpoint.NewFromFloat(sma.Index(1))).Sub(s.IncreasePriceScale) < fixedpoint.Zero {
		logrus.Debug("未達成-IncreasePriceScale")
		return
	}

	//K線本身品質檢查
	if kline.GetAmplification().Sub(s.AmplificationPercent) < fixedpoint.Zero {
		//波動超過X
		logrus.Debug("未達成-波動超過X")
		return
	}

	if kline.GetAmplification().Sub(s.OverAmplificationPercent) > fixedpoint.Zero {
		//波動太超過，就剔除
		logrus.Debug("未達成-波動平穩，就剔除")
		return
	}

	// 實K要超過特定比例
	if kline.GetThickness().Sub(s.ChangeRatio) < fixedpoint.Zero {
		logrus.Debug("未達成-實K要超過特定比例")
		return
	}

	// 向上力道要超過特定比例
	if kline.GetUpperPowerRatio().Sub(s.UpperPowerRatio) < fixedpoint.Zero {
		logrus.Debug("未達成-向上力道要超過特定比例")
		return
	}

	//上影線要小於特定比例
	if fixedpoint.One.Sub(kline.GetUpperShadowRatio()).Sub(s.UpperShadowRatio) < fixedpoint.Zero {
		logrus.Debug("未達成-上影線要小於特定比例")
		return
	}

	killedKinfos := last.KilledKDatas
	logrus.Debug(killedKinfos)
	rangedKInfos := killedKinfos.GetWidthRange(s.WinLeftCount, s.WinRightCount, s.WinLeftCount*s.WinMaxMul, s.WinRightCount*s.WinMaxMul)
	logrus.Debug(rangedKInfos)
	lowerRightKInfos := rangedKInfos.GetLeftLowerRight(s.AllowRightUpPercent)
	logrus.Debug(lowerRightKInfos)
	tempKInfos := lowerRightKInfos.GetSumWidthLargeThan(s.SumWidthMin)
	logrus.Debug(tempKInfos)

	if tempKInfos.Length() != 0 { // canBuy
		s.lowerHighTimes = 0

		if !s.HasPosition() {

			// order
			orderUSD := s.initialUsd

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
			orderUSD = orderUSD.Mul(s.leverage)

			quantity := orderUSD.Div(kline.Close) //fixedpoint.NewFromFloat(0.01)

			//設定 Tag資訊
			tempKInfo := tempKInfos.GetSumLoseMin()
			tag := fmt.Sprintf("%d-%d-%d-%d", tempKInfo.LoseLeftIndex, tempKInfo.LoseRightIndex, killedKinfos.Length(), rangedKInfos.Length())

			//執行購買
			_, err := orderExecutor.SubmitOrders(ctx, types.SubmitOrder{
				Symbol:           kline.Symbol,
				Market:           market,
				Side:             types.SideTypeBuy,
				Type:             types.OrderTypeMarket,
				Quantity:         quantity,
				MarginSideEffect: types.SideEffectTypeMarginBuy,
				Tag:              tag,
			})
			if err != nil {
				log.WithError(err).Error("subit buy order error")
			}
			s.positionKline = kline
			s.lastOrderQuantity = quantity
			s.klineLow = kline.Low

		} else {
			log.Infoln("already has position")
		}
	}
}

func (s *WChartTactic) HasPosition() bool {
	return s.positionKline.Volume != fixedpoint.Zero
}