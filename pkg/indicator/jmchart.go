package indicator

import (
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

//go:generate callbackgen -type JMChart
type JMChart struct {
	types.SeriesBase
	types.IntervalWindow

	Values KDatas

	EndTime         time.Time
	UpdateCallbacks []func(value KData)
}

func (inc *JMChart) Update(currentKline types.KLine) {

	KData := KData{}
	KData.K = currentKline
	inc.setMChart(&KData, currentKline.Low)

	inc.Values = append(inc.Values, KData)
}

func (inc *JMChart) setMChart(KData *KData, currentLowPrice fixedpoint.Value) {
	maxIndex := inc.Values.Length() - 1
	jumpSize := 1
	killedKDatas := KDatas{}
	highestPrice := fixedpoint.Zero
	// 比較整個數據
	for i := maxIndex; i >= 0; i = i - jumpSize {
		v := &inc.Values[i]

		// 先計算是否更新最大值
		if v.K.High > highestPrice {
			highestPrice = v.K.High
		}

		// 輸贏index處理
		if currentLowPrice > v.K.Low {
			KData.LoseLeftIndex += 1
			break
		}

		//把已經擊倒的K線數量設定在此處
		v.LoseRightIndex = KData.LoseLeftIndex + 1
		v.RightCuspPrice = highestPrice
		killedKDatas = append(killedKDatas, *v)

		jumpSize = v.LoseLeftIndex
		KData.LoseLeftIndex += v.LoseLeftIndex
		if v.LoseLeftIndex == 0 || KData.LoseLeftIndex >= maxIndex { //表示第一個 or 超過第一個
			KData.LoseLeftIndex += 1
			break
		}
	}

	KData.LeftCuspPrice = highestPrice
	KData.KilledKDatas = killedKDatas
}

func (inc *JMChart) Index(i int) KData {
	if inc.Values == nil {
		return KData{}
	}
	return inc.Values.Index(i)
}

func (inc *JMChart) Length() int {
	if inc.Values == nil {
		return 0
	}
	return inc.Values.Length()
}

func (inc *JMChart) Last() KData {
	if inc.Values == nil {
		return KData{}
	}
	return inc.Values.Last()
}

func (inc *JMChart) IndexWidth(index, width int) KDatas {
	if inc.Values == nil {
		return KDatas{}
	}
	return inc.Values.IndexWidth(index, width)
}

// interfaces implementation check
//var _ Simple = &JMChart{}
//var _ types.SeriesExtend = &JMChart{}

func (inc *JMChart) PushK(k types.KLine) {

	if inc.EndTime != zeroTime && k.EndTime.Before(inc.EndTime) {
		return
	}

	inc.Update(k)
	inc.EndTime = k.EndTime.Time()
	inc.EmitUpdate(inc.Values.Last())
}
