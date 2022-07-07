// package lmsr implements a Logarithmic Market Scoring Rule

package lmsr

import "math"

const Liquidity = float64(10.0)

// type Stock {
// 	OutstandingShares int
// }

// Price calculates the price of a stock given a liquidity constant (b),
// the number of outstanding shares for this stock (shares) and the number
// of outstanding shares for all stocks, represented as an array.
func Price(b float64, shares float64, allShares []float64) float64 {
	num := math.Exp(shares / b)

	sum := float64(0)
	for _, s := range allShares {
		sum += math.Exp(s / b)
	}
	return num / sum
}

// TradeCost calculates the price of buying `shares` shares of a stock, given
// a liquidity constant b, the outstanding shares for all stocks, and the
// index of our particular stock in this array of outstanding shares.
func TradeCost(b float64, shares float64, allShares []float64, idx int) float64 {
	costBefore := cost(b, allShares)
	allShares[idx] += shares
	costAfter := cost(b, allShares)
	return costAfter - costBefore
}

func cost(b float64, allShares []float64) float64 {
	sum := float64(0)
	for _, s := range allShares {
		sum += math.Exp(s / b)
	}
	return b * math.Log(sum)
}
