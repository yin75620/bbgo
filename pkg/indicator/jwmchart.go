package indicator

import (
	"math"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type KInfo struct {
	HighLoseLeftIndex  int              //高點往左數，在第幾個位置高點比人低，第一根K線就是1
	HighLoseRightIndex int              //高點往右數，在第幾個位置高點比人低
	LeftLowestPrice    fixedpoint.Value //高點往左的最底點價格
	RightLowestPrice   fixedpoint.Value //高點往右的最底點價格
	//LowLoseIndex       int         //低點往左數，在第幾個位置低點比人高
	K types.KLine // include id, high, low,

	//IsKillTopKline bool
	WKilledKInfos KInfos

	LowLoseLeftIndex  int              //低點往左數，第幾個位置低點比人高，第一根就是1
	LowLoseRightIndex int              //低點往右數，第幾個位置低點比人高
	LeftHighestPrice  fixedpoint.Value //中央低點往左的最高點
	RightHighestPrice fixedpoint.Value //中央低點往右的最高點
	MKilledKIinfos    KInfos
}

type KBunch struct {
	KInfo

	KilledKInfos KInfos
}

type KInfos []KInfo

func (ks *KInfos) Last() KInfo {
	length := len(*ks)
	if length > 0 {
		return (*ks)[length-1]
	}
	return KInfo{}
}

func (s *KInfos) Index(i int) KInfo {
	length := len(*s)
	if length-i <= 0 || i < 0 {
		return KInfo{}
	}
	return (*s)[length-i-1]
}

func (s *KInfos) Length() int {
	return len(*s)
}

func (s *KInfos) IndexWidth(i int, width int) KInfos {
	length := len(*s)
	if length-i <= 0 || i < 0 {
		return KInfos{}
	}
	leftSide := length - i - 1 - width
	if leftSide < 0 {
		leftSide = 0
	}
	return (*s)[leftSide : length-i-1]
}

func (s *KInfos) GetWidthRange(left int, right int, leftMax, rightMax int) KInfos {
	length := len(*s)
	if length <= 0 {
		return *s
	}
	res := KInfos{}
	for _, v := range *s {
		if v.HighLoseLeftIndex > left && v.HighLoseRightIndex > right &&
			v.HighLoseLeftIndex < leftMax && v.HighLoseRightIndex < rightMax {
			res = append(res, v)
		}
	}
	return res
}

func (s *KInfos) GetSumLoseTop() KInfo {
	length := len(*s)
	res := KInfo{}
	if length <= 0 {
		return res
	}
	sumLoseMax := 0
	for _, v := range *s {
		sum := v.HighLoseLeftIndex + v.HighLoseRightIndex
		if sum > sumLoseMax {
			sumLoseMax = sum
			res = v
		}
	}
	return res
}

func (s *KInfos) GetSumLoseMin() KInfo {
	length := len(*s)
	res := KInfo{}
	if length <= 0 {
		return res
	}
	sumLoseMin := math.MaxInt32
	for _, v := range *s {
		sum := v.HighLoseLeftIndex + v.HighLoseRightIndex
		if sum < sumLoseMin {
			sumLoseMin = sum
			res = v
		}
	}
	return res
}

func (s *KInfos) GetLeftLowerRight(allowRightUpPercent float64) KInfos {
	length := len(*s)
	if length <= 0 {
		return *s
	}
	res := KInfos{}
	for _, v := range *s {
		if v.LeftLowestPrice < v.RightLowestPrice.Mul(fixedpoint.NewFromFloat(1+allowRightUpPercent)) {
			res = append(res, v)
		}
	}
	return res
}

func (s *KInfos) GetHighLoseLeftIndexLargerThan(minIndex int) KInfos {
	length := len(*s)
	if length <= 0 {
		return *s
	}
	res := KInfos{}
	for _, v := range *s {
		if v.HighLoseLeftIndex > minIndex && v.HighLoseRightIndex == 0 {
			res = append(res, v)
		}
	}
	return res
}

func (s *KInfos) GetSumWidthLargeThan(widthMin int) KInfos {
	length := len(*s)
	res := KInfos{}
	if length <= 0 {
		return res
	}

	for _, v := range *s {
		sum := v.HighLoseLeftIndex + v.HighLoseRightIndex
		if sum > widthMin {
			res = append(res, v)
		}
	}
	return res
}

//go:generate callbackgen -type JWMChart
type JWMChart struct {
	types.SeriesBase
	types.IntervalWindow

	// Setting 高點要贏左右各多少個K線才算合格
	WinLeftCount        int
	WinRightCount       int
	AllowRightUpPercent float64

	Values KInfos

	EndTime         time.Time
	UpdateCallbacks []func(value KInfo)
}

func (inc *JWMChart) Update(currentKline types.KLine) {

	kinfo := KInfo{}
	kinfo.K = currentKline
	inc.setWChart(&kinfo, currentKline.High)

	inc.Values = append(inc.Values, kinfo)
}

func (inc *JWMChart) setWChart(kinfo *KInfo, currentHighPrice fixedpoint.Value) {
	maxIndex := inc.Values.Length() - 1
	jumpSize := 1
	killedKInfos := KInfos{}
	lowestPrice := fixedpoint.NewFromFloat(math.MaxFloat64)
	// 比較整個數據
	for i := maxIndex; i >= 0; i = i - jumpSize {
		v := &inc.Values[i]

		// 先計算是否更新最小值
		if v.K.Low < lowestPrice {
			lowestPrice = v.K.Low
		}

		// 輸贏index處理
		if currentHighPrice < v.K.High {
			kinfo.HighLoseLeftIndex += 1
			break
		}

		//把已經擊倒的K線數量設定在此處
		v.HighLoseRightIndex = kinfo.HighLoseLeftIndex + 1
		v.RightLowestPrice = lowestPrice
		killedKInfos = append(killedKInfos, *v)

		jumpSize = v.HighLoseLeftIndex
		kinfo.HighLoseLeftIndex += v.HighLoseLeftIndex
		if v.HighLoseLeftIndex == 0 || kinfo.HighLoseLeftIndex >= maxIndex { //表示第一個 or 超過第一個
			kinfo.HighLoseLeftIndex += 1
			break
		}
	}

	kinfo.LeftLowestPrice = lowestPrice
	//tempKInfos := killedKInfos.GetWidthRange(inc.WinLeftCount, inc.WinRightCount, 10000, 10000)
	//tempKInfos = tempKInfos.GetLeftLowerRight(inc.AllowRightUpPercent)

	// 取出這次擊倒的最大KInfo
	//topKInfo := tempKInfos.GetSumLoseTop()

	//kinfo.IsKillTopKline = topKInfo.HighLoseRightIndex != 0
	kinfo.WKilledKInfos = killedKInfos
}

func (inc *JWMChart) setMChart(kinfo *KInfo, currentHighPrice fixedpoint.Value) {

}

func (inc *JWMChart) Index(i int) KInfo {
	if inc.Values == nil {
		return KInfo{}
	}
	return inc.Values.Index(i)
}

func (inc *JWMChart) Length() int {
	if inc.Values == nil {
		return 0
	}
	return inc.Values.Length()
}

func (inc *JWMChart) Last() KInfo {
	if inc.Values == nil {
		return KInfo{}
	}
	return inc.Values.Last()
}

func (inc *JWMChart) IndexWidth(index, width int) KInfos {
	if inc.Values == nil {
		return KInfos{}
	}
	return inc.Values.IndexWidth(index, width)
}

// interfaces implementation check
//var _ Simple = &JWMChart{}
//var _ types.SeriesExtend = &JWMChart{}

func (inc *JWMChart) PushK(k types.KLine) {

	if inc.EndTime != zeroTime && k.EndTime.Before(inc.EndTime) {
		return
	}

	inc.Update(k)
	inc.EndTime = k.EndTime.Time()
	inc.EmitUpdate(inc.Values.Last())
}
