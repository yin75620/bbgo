package indicator

import (
	"math"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type KData struct {
	K types.KLine // include id, high, low,

	LoseLeftIndex  int              //低點往左數，第幾個位置低點比人高，第一根就是1
	LoseRightIndex int              //低點往右數，第幾個位置低點比人高
	LeftCuspPrice  fixedpoint.Value //中央低點往左的最高點
	RightCuspPrice fixedpoint.Value //中央低點往右的最高點
	KilledKDatas   KDatas
}

type KDatas []KData

func (ks *KDatas) Last() KData {
	length := len(*ks)
	if length > 0 {
		return (*ks)[length-1]
	}
	return KData{}
}

func (s *KDatas) Index(i int) KData {
	length := len(*s)
	if length-i <= 0 || i < 0 {
		return KData{}
	}
	return (*s)[length-i-1]
}

func (s *KDatas) Length() int {
	return len(*s)
}

func (s *KDatas) IndexWidth(i int, width int) KDatas {
	length := len(*s)
	if length-i <= 0 || i < 0 {
		return KDatas{}
	}
	leftSide := length - i - 1 - width
	if leftSide < 0 {
		leftSide = 0
	}
	return (*s)[leftSide : length-i-1]
}

func (s *KDatas) GetWidthRange(left int, right int, leftMax, rightMax int) KDatas {
	length := len(*s)
	if length <= 0 {
		return *s
	}
	res := KDatas{}
	for _, v := range *s {
		if v.LoseLeftIndex > left && v.LoseRightIndex > right &&
			v.LoseLeftIndex < leftMax && v.LoseRightIndex < rightMax {
			res = append(res, v)
		}
	}
	return res
}

func (s *KDatas) GetSumLoseTop() KData {
	length := len(*s)
	res := KData{}
	if length <= 0 {
		return res
	}
	sumLoseMax := 0
	for _, v := range *s {
		sum := v.LoseLeftIndex + v.LoseRightIndex
		if sum > sumLoseMax {
			sumLoseMax = sum
			res = v
		}
	}
	return res
}

func (s *KDatas) GetSumLoseMin() KData {
	length := len(*s)
	res := KData{}
	if length <= 0 {
		return res
	}
	sumLoseMin := math.MaxInt32
	for _, v := range *s {
		sum := v.LoseLeftIndex + v.LoseRightIndex
		if sum < sumLoseMin {
			sumLoseMin = sum
			res = v
		}
	}
	return res
}

func (s *KDatas) GetLeftLowerRight(allowRightUpPercent float64) KDatas {
	length := len(*s)
	if length <= 0 {
		return *s
	}
	res := KDatas{}
	for _, v := range *s {
		if v.LeftCuspPrice < v.RightCuspPrice.Mul(fixedpoint.NewFromFloat(1+allowRightUpPercent)) {
			res = append(res, v)
		}
	}
	return res
}

func (s *KDatas) GetLeftHigherRight(allowLeftUpPercent float64) KDatas {
	length := len(*s)
	if length <= 0 {
		return *s
	}
	res := KDatas{}
	for _, v := range *s {
		if v.LeftCuspPrice.Mul(fixedpoint.NewFromFloat(1+allowLeftUpPercent)) > v.RightCuspPrice {
			res = append(res, v)
		}
	}
	return res
}

func (s *KDatas) GetLoseLeftIndexLargerThan(minIndex int) KDatas {
	length := len(*s)
	if length <= 0 {
		return *s
	}
	res := KDatas{}
	for _, v := range *s {
		if v.LoseLeftIndex > minIndex && v.LoseRightIndex == 0 {
			res = append(res, v)
		}
	}
	return res
}

func (s *KDatas) GetSumWidthLargeThan(widthMin int) KDatas {
	length := len(*s)
	res := KDatas{}
	if length <= 0 {
		return res
	}

	for _, v := range *s {
		sum := v.LoseLeftIndex + v.LoseRightIndex
		if sum > widthMin {
			res = append(res, v)
		}
	}
	return res
}
