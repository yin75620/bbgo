package jeffmw

import (
	"fmt"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/sirupsen/logrus"
)

type MChartTactic struct {
	//configUsdValue fixedpoint.Value

	//setting
	LimithigherLowTimes int              `json:"limitLowerHightTimes"`
	initialUsd          fixedpoint.Value //`json:"initialUsd"` //if 0 => 100
	leverage            fixedpoint.Value //`json:"leverage"`   // if 0 => 1
	WinLeftCount        int              `json:"winLeftCount"`
	WinRightCount       int              `json:"winRightCount"`
	SumWidthMin         int              `json:"sumWidthMin"`
	WinMaxMul           int              `json:"winMaxMul"`
	AllowLeftUpPercent  float64          `json:"allowLeftUpPercent"` //0.012就表示右邊低點往上1.012倍後會比左邊低點高
	IsCompoundOrder     bool             `json:"isCompoundOrder"`

	//
	IncreaseVolScale     fixedpoint.Value `json:"increaseVolScale"`
	IncreasePriceScale   fixedpoint.Value `json:"increasePriceScale"`
	GainVolPreDayScale   fixedpoint.Value `json:"gainVolPreDayScale"`
	AmplificationPercent fixedpoint.Value `json:"amplificationPercent"` //大於
	ChangeRatio          fixedpoint.Value `json:"changeRatio"`          //大於
	LowerPowerRatio      fixedpoint.Value `json:"lowerPowerRatio"`      //大於
	LowerShadowRatio     fixedpoint.Value `json:"lowerShadowRatio"`     //小於

	OverAmplificationPercent fixedpoint.Value `json:"overAmplificationPercent"` //小於

	ForwardWidth     int `json:"ForwardWidth"`
	LoseLeftIndexMin int `json:"LoseLeftIndexMin"`

	// start info
	configUsdValue    fixedpoint.Value
	positionKline     types.KLine
	higherLowTimes    int
	lastOrderQuantity fixedpoint.Value
	klineHigh         fixedpoint.Value

	repeater *Repeater
}

func NewMChartTactic() *MChartTactic {
	return &MChartTactic{}
}

func (mct *MChartTactic) Init(s *Strategy, repeater *Repeater) {
	if mct.WinMaxMul == 0 {
		mct.WinMaxMul = 1000
	}

	mct.positionKline = types.KLine{}
	mct.configUsdValue = s.configUsdValue

	mct.initialUsd = s.InitialUsd
	mct.leverage = s.Leverage

	mct.repeater = repeater

}

func (mct *MChartTactic) OnKLineClosed(kline types.KLine) {
	repeater := mct.repeater
	jmchart := repeater.jmchart
	vma := repeater.vma
	sma := repeater.sma
	session := repeater.session
	ctx := repeater.ctx
	orderExecutor := repeater.orderExecutor
	market := repeater.market

	last := jmchart.Last()
	if mct.HasPosition() { //prepare sell

		//止損策略

		if last.LoseLeftIndex == 1 {
			mct.higherLowTimes += 1
		}

		//Stop Lose
		if mct.higherLowTimes > mct.LimithigherLowTimes {
			mct.PositionClose(kline)
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
	if kline.Volume.Div(fixedpoint.NewFromFloat(vma.Index(1))).Sub(mct.IncreaseVolScale) < fixedpoint.Zero {
		logrus.Debug("未達成-成交量的/超越均量指定比例")
		return
	}

	//成交量比前一根高出指定比例
	if kline.Volume.Div(jmchart.Index(1).K.Volume).Sub(mct.GainVolPreDayScale) < fixedpoint.Zero {
		logrus.Debug("未達成-成交量比前一根高出指定比例")
		return
	}

	//找到輸掉的那一根Ｋ線，再往前N跟，如果有出現尖頭，也不交易
	if mct.LoseLeftIndexMin != 0 {
		leftSideKDatas := jmchart.IndexWidth(last.LoseLeftIndex, mct.ForwardWidth)
		topKDatas := leftSideKDatas.GetLoseLeftIndexLargerThan(mct.LoseLeftIndexMin)

		if len(topKDatas) != 0 {
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
	if fixedpoint.NewFromFloat(sma.Index(1)).Div(kline.Close).Sub(mct.IncreasePriceScale) < fixedpoint.Zero {
		logrus.Debug("未達成-IncreasePriceScale")
		return
	}

	//K線本身品質檢查
	if kline.GetAmplification().Sub(mct.AmplificationPercent) < fixedpoint.Zero {
		//波動超過X
		logrus.Debug("未達成-波動超過X")
		return
	}

	if kline.GetAmplification().Sub(mct.OverAmplificationPercent) > fixedpoint.Zero {
		//波動太超過，就剔除
		logrus.Debug("未達成-波動平穩，就剔除")
		return
	}

	// 實K要超過特定比例
	if kline.GetThickness().Sub(mct.ChangeRatio) < fixedpoint.Zero {
		logrus.Debug("未達成-實K要超過特定比例")
		return
	}

	// 向下力道要超過特定比例
	if kline.GetLowerPowerRatio().Sub(mct.LowerPowerRatio) < fixedpoint.Zero {
		logrus.Debug("未達成-向上力道要超過特定比例")
		return
	}

	//上影線要小於特定比例
	if fixedpoint.One.Sub(kline.GetLowerShadowRatio()).Sub(mct.LowerShadowRatio) < fixedpoint.Zero {
		logrus.Debug("未達成-上影線要小於特定比例")
		return
	}

	killedKDatas := last.KilledKDatas
	rangedKDatas := killedKDatas.GetWidthRange(mct.WinLeftCount, mct.WinRightCount, mct.WinLeftCount*mct.WinMaxMul, mct.WinRightCount*mct.WinMaxMul)
	lowerRightKDatas := rangedKDatas.GetLeftHigherRight(mct.AllowLeftUpPercent)
	tempKDatas := lowerRightKDatas.GetSumWidthLargeThan(mct.SumWidthMin)

	if tempKDatas.Length() != 0 { // canSell
		mct.higherLowTimes = 0

		if !mct.HasPosition() {

			// order
			orderUSD := mct.initialUsd

			// money check
			usdtBalance, _ := session.Account.Balance("USDT")
			revenue := usdtBalance.Total().Sub(mct.configUsdValue)
			totalAvalableUSD := orderUSD.Add(revenue)

			if totalAvalableUSD < 0 {
				//money not enough
				return
			}

			if mct.IsCompoundOrder {
				orderUSD = totalAvalableUSD
			}

			//計算要下單的數量
			orderUSD = orderUSD.Mul(mct.leverage)

			quantity := orderUSD.Div(kline.Close) //fixedpoint.NewFromFloat(0.01)

			//設定 Tag資訊
			tempKInfo := tempKDatas.GetSumLoseMin()
			tag := fmt.Sprintf("%d-%d-%d-%d", tempKInfo.LoseLeftIndex, tempKInfo.LoseRightIndex, killedKDatas.Length(), rangedKDatas.Length())

			//執行放空開倉
			_, err := orderExecutor.SubmitOrders(ctx, types.SubmitOrder{
				Symbol:           kline.Symbol,
				Market:           market,
				Side:             types.SideTypeSell,
				Type:             types.OrderTypeMarket,
				Quantity:         quantity,
				MarginSideEffect: types.SideEffectTypeMarginBuy,
				Tag:              tag,
			})
			if err != nil {
				log.WithError(err).Error("subit buy order error")
			}
			mct.positionKline = kline
			mct.lastOrderQuantity = quantity
			mct.klineHigh = kline.High

		} else {
			log.Infoln("already has position")
		}
	}
}

func (mct *MChartTactic) HasPosition() bool {
	return mct.positionKline.Volume != fixedpoint.Zero
}

func (mct *MChartTactic) PositionClose(kline types.KLine) {

	if !mct.HasPosition() {
		logrus.Info("already close")
		return
	}

	orderExecutor := mct.repeater.orderExecutor
	ctx := mct.repeater.ctx
	market := mct.repeater.market

	//買回關倉
	_, err := orderExecutor.SubmitOrders(ctx, types.SubmitOrder{
		Symbol:           kline.Symbol,
		Market:           market,
		Side:             types.SideTypeBuy,
		Type:             types.OrderTypeMarket,
		Quantity:         mct.lastOrderQuantity,
		MarginSideEffect: types.SideEffectTypeAutoRepay,
	})
	if err != nil {
		log.WithError(err).Error("subit sell order error")
	}
	mct.positionKline = types.KLine{}
	mct.lastOrderQuantity = fixedpoint.Zero
	mct.klineHigh = fixedpoint.Zero

}
