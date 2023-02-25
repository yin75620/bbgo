package indicator

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

/*
python:

import pandas as pd
import pandas_ta as ta

data = pd.Series([0,1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,9,0,1,2,3,4,5,6,7,8,9,0])
size = 5

result = ta.XMA(data, size)
print(result)
*/
func Test_XMA(t *testing.T) {
	Delta := 0.001
	var randomPrices = []byte(`[0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9]`)
	var input []fixedpoint.Value
	if err := json.Unmarshal(randomPrices, &input); err != nil {
		panic(err)
	}
	tests := []struct {
		name         string
		kLines       []types.KLine
		want         float64
		next         float64
		update       float64
		updateResult float64
		all          int
	}{
		{
			name:         "test",
			kLines:       buildKLines(input),
			want:         7.0,
			next:         6.0,
			update:       0,
			updateResult: 6.0,
			all:          27,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			XMA := XMA{
				IntervalWindow: types.IntervalWindow{Window: 5},
			}

			for _, k := range tt.kLines {
				XMA.PushK(k)
			}

			assert.InDelta(t, tt.want, XMA.Last(), Delta)
			assert.InDelta(t, tt.next, XMA.Index(1), Delta)
			XMA.Update(tt.update)
			assert.InDelta(t, tt.updateResult, XMA.Last(), Delta)
			assert.Equal(t, tt.all, XMA.Length())
		})
	}
}
