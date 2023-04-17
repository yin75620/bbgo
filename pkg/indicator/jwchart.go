package indicator

import (
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

//go:generate callbackgen -type JWChart
type JWChart struct {
	types.SeriesBase
	types.IntervalWindow

	Values KDatas

	EndTime         time.Time
	UpdateCallbacks []func(value KData)
}

func (inc *JWChart) Update(currentKline types.KLine) {

	KData := KData{}
	KData.K = currentKline
	inc.setWChart(&KData, currentKline.High)

	inc.Values = append(inc.Values, KData)
}

func (inc *JWChart) setWChart(KData *KData, currentHighPrice fixedpoint.Value) {
	maxIndex := inc.Values.Length() - 1
	jumpSize := 1
	killedKDatas := KDatas{}
	lowestPrice := fixedpoint.PosInf
	// 比較整個數據
	for i := maxIndex; i >= 0; i = i - jumpSize {
		v := &inc.Values[i]

		// 先計算是否更新最小值
		if v.K.Low < lowestPrice {
			lowestPrice = v.K.Low
		}

		// 輸贏index處理
		if currentHighPrice < v.K.High {
			KData.LoseLeftIndex += 1
			break
		}

		//把已經擊倒的K線數量設定在此處
		v.LoseRightIndex = KData.LoseLeftIndex + 1
		v.RightCuspPrice = lowestPrice
		killedKDatas = append(killedKDatas, *v)

		jumpSize = v.LoseLeftIndex
		KData.LoseLeftIndex += v.LoseLeftIndex
		if v.LoseLeftIndex == 0 || KData.LoseLeftIndex >= maxIndex { //表示第一個 or 超過第一個
			KData.LoseLeftIndex += 1
			break
		}
	}

	KData.LeftCuspPrice = lowestPrice
	KData.KilledKDatas = killedKDatas
}

func (inc *JWChart) Index(i int) KData {
	if inc.Values == nil {
		return KData{}
	}
	return inc.Values.Index(i)
}

func (inc *JWChart) Length() int {
	if inc.Values == nil {
		return 0
	}
	return inc.Values.Length()
}

func (inc *JWChart) Last() KData {
	if inc.Values == nil {
		return KData{}
	}
	return inc.Values.Last()
}

func (inc *JWChart) IndexWidth(index, width int) KDatas {
	if inc.Values == nil {
		return KDatas{}
	}
	return inc.Values.IndexWidth(index, width)
}

// interfaces implementation check
//var _ Simple = &JWChart{}
//var _ types.SeriesExtend = &JWChart{}

func (inc *JWChart) PushK(k types.KLine) {

	if inc.EndTime != zeroTime && k.EndTime.Before(inc.EndTime) {
		return
	}

	inc.Update(k)
	inc.EndTime = k.EndTime.Time()
	inc.EmitUpdate(inc.Values.Last())
}
