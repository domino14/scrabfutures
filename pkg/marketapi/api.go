package marketapi

import (
	"context"

	pb "github.com/domino14/scrabfutures/rpc/proto"
)

type MarketService struct{}

func NewMarketService() *MarketService {
	return &MarketService{}
}

func (m *MarketService) GetOrderBook(ctx context.Context, req *pb.GetOrderBookRequest) (*pb.OrderBookResponse, error) {

	return nil, nil
}
