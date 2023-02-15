package jeff1

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "jeff1"

var log = logrus.WithField("strategy", ID)
var cpiTimeUnix = time.Time{}.UnixMilli()

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type Strategy struct {
	Symbol        string               `json:"symbol"`
	MovingAverage types.IntervalWindow `json:"movingAverage"`
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: "1m"})
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
		// skip k-lines from other symbols
		if kline.Symbol != s.Symbol {
			return
		}

		log.Infof("%s", kline.String())

	})

	session.MarketDataStream.OnKLine(func(kline types.KLine) {
		// 在公布CPI後的一秒內方向，就順勢交易
		// 往反方向走出大量or開倉or幅度的1/5

		// 把所有的CPI時間都裝進Array，用來回測。 之後改抓取fred的api數據實作

		now := time.Now().UnixMilli()

		if kline.StartTime.UnixMilli() < cpiTimeUnix || kline.StartTime.UnixMilli() > cpiTimeUnix {
			log.Infof("wait 1 sec")
			return
		}

		//表示cpi當下，用一秒鐘判斷方向
		if now-kline.StartTime.UnixMilli() <= 1000 {
			log.Infof("wait 1 sec")
			return
		}

		// 檢查有無持倉
		hasPosition := false
		if !hasPosition {
			//一秒後，檢查 k線往哪個方向
			d := kline.Direction()
			switch d {
			case types.DirectionUp:
				//買進
			case types.DirectionDown:
				//賣出
			}
		} else {
			//反向或大量就賣
		}

	})

	return nil
}

// InstanceID returns the instance identifier from the current grid configuration parameters
func (s *Strategy) InstanceID() string {
	return fmt.Sprintf("%s-%s", ID, s.Symbol)
}
