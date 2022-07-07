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
	is.True(withinEpsilon(Price(10, 10, []float64{10, 20, 23}), 0.13536))
}

func TestTradeCost(t *testing.T) {
	is := is.New(t)
	is.True(withinEpsilon(TradeCost(10, 7, []float64{10, 20, 23}, 0), 1.28590))
}
