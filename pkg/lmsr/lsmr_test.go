package lmsr

import (
	"math"
	"testing"

	"github.com/matryer/is"
)

const Epsilon = 1e-5

func withinEpsilon(a, b float64) bool {
	return math.Abs(a-b) < Epsilon
}

func TestPrice(t *testing.T) {
	is := is.New(t)
	is.True(withinEpsilon(Price(10, []float64{10, 20, 23}, 0), 13.536235))
}

func TestPrice2(t *testing.T) {
	is := is.New(t)
	is.True(withinEpsilon(Price(100, []float64{100, 200, 230}, 0), 13.536235))
}

func TestPriceEmptyShares(t *testing.T) {
	is := is.New(t)
	is.True(withinEpsilon(Price(100, []float64{0, 0, 0, 0, 0, 0, 0}, 2), 100/7.0))
}

func TestTradeCost(t *testing.T) {
	is := is.New(t)
	is.True(withinEpsilon(TradeCost(10, 7, []float64{10, 20, 23}, 0), 128.590162))
}

func TestTradeCost2(t *testing.T) {
	is := is.New(t)
	is.True(withinEpsilon(TradeCost(100, 70, []float64{100, 200, 230}, 0), 1285.90162))
}

func TestTradeCost3(t *testing.T) {
	is := is.New(t)
	is.True(withinEpsilon(TradeCost(100, 50, []float64{0, 0, 0, 0}, 0),
		1502.978252))

}

// func TestTradeCost4(t *testing.T) {
// 	is := is.New(t)
// 	fmt.Println(TradeCost(100, 50, []float64{300, 300, 300, 300}, 2))
// 	is.True(false)
// }
