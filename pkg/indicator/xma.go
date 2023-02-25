package indicator

import (
	"time"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
)

const MaxNumOfXMA = 5_000
const MaxNumOfXMATruncateSize = 100

type KlinePartFunc func(types.KLine) float64

//go:generate callbackgen -type XMA
type XMA struct {
	types.SeriesBase
	types.IntervalWindow

	KlinePart KlinePartFunc
	Values    floats.Slice
	rawValues *types.Queue
	EndTime   time.Time

	UpdateCallbacks []func(value float64)
}

func (inc *XMA) Last() float64 {
	if inc.Values.Length() == 0 {
		return 0.0
	}
	return inc.Values.Last()
}

func (inc *XMA) Index(i int) float64 {
	if i >= inc.Values.Length() {
		return 0.0
	}

	return inc.Values.Index(i)
}

func (inc *XMA) Length() int {
	return inc.Values.Length()
}

func (inc *XMA) Clone() types.UpdatableSeriesExtend {
	out := &XMA{
		Values:    inc.Values[:],
		rawValues: inc.rawValues.Clone(),
		EndTime:   inc.EndTime,
	}
	out.SeriesBase.Series = out
	return out
}

var _ types.SeriesExtend = &XMA{}

func (inc *XMA) Update(value float64) {
	if inc.rawValues == nil {
		inc.rawValues = types.NewQueue(inc.Window)
		inc.SeriesBase.Series = inc
	}

	inc.rawValues.Update(value)
	if inc.rawValues.Length() < inc.Window {
		return
	}

	inc.Values.Push(types.Mean(inc.rawValues))
	if len(inc.Values) > MaxNumOfXMA {
		inc.Values = inc.Values[MaxNumOfXMATruncateSize-1:]
	}
}

func (inc *XMA) BindK(target KLineClosedEmitter, symbol string, interval types.Interval) {
	target.OnKLineClosed(types.KLineWith(symbol, interval, inc.PushK))
}

func (inc *XMA) PushK(k types.KLine) {
	if inc.EndTime != zeroTime && k.EndTime.Before(inc.EndTime) {
		return
	}

	res := k.Close.Float64()
	if inc.KlinePart != nil {
		res = inc.KlinePart(k)
	}

	inc.Update(res)
	inc.EndTime = k.EndTime.Time()
	inc.EmitUpdate(inc.Values.Last())
}

func (inc *XMA) LoadK(allKLines []types.KLine) {
	for _, k := range allKLines {
		inc.PushK(k)
	}
}
