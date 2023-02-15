package indicator

import (
	"time"

	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/types"
)

const MaxNumOfVMA = 5_000
const MaxNumOfVMATruncateSize = 100

//go:generate callbackgen -type VMA
type VMA struct {
	types.SeriesBase
	types.IntervalWindow
	Values    floats.Slice
	rawValues *types.Queue
	EndTime   time.Time

	UpdateCallbacks []func(value float64)
}

func (inc *VMA) Last() float64 {
	if inc.Values.Length() == 0 {
		return 0.0
	}
	return inc.Values.Last()
}

func (inc *VMA) Index(i int) float64 {
	if i >= inc.Values.Length() {
		return 0.0
	}

	return inc.Values.Index(i)
}

func (inc *VMA) Length() int {
	return inc.Values.Length()
}

func (inc *VMA) Clone() types.UpdatableSeriesExtend {
	out := &SMA{
		Values:    inc.Values[:],
		rawValues: inc.rawValues.Clone(),
		EndTime:   inc.EndTime,
	}
	out.SeriesBase.Series = out
	return out
}

var _ types.SeriesExtend = &VMA{}

func (inc *VMA) Update(value float64) {
	if inc.rawValues == nil {
		inc.rawValues = types.NewQueue(inc.Window)
		inc.SeriesBase.Series = inc
	}

	inc.rawValues.Update(value)
	if inc.rawValues.Length() < inc.Window {
		return
	}

	inc.Values.Push(types.Mean(inc.rawValues))
	if len(inc.Values) > MaxNumOfSMA {
		inc.Values = inc.Values[MaxNumOfSMATruncateSize-1:]
	}
}

func (inc *VMA) BindK(target KLineClosedEmitter, symbol string, interval types.Interval) {
	target.OnKLineClosed(types.KLineWith(symbol, interval, inc.PushK))
}

func (inc *VMA) PushK(k types.KLine) {
	if inc.EndTime != zeroTime && k.EndTime.Before(inc.EndTime) {
		return
	}

	inc.Update(k.Volume.Float64())
	inc.EndTime = k.EndTime.Time()
	inc.EmitUpdate(inc.Values.Last())
}

func (inc *VMA) LoadK(allKLines []types.KLine) {
	for _, k := range allKLines {
		inc.PushK(k)
	}
}
