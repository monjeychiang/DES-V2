package strategy

import (
	"context"
	"time"

	pb "trading-core/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// WorkerClient sends ticks to the Python worker over gRPC.
type WorkerClient struct {
	conn   *grpc.ClientConn
	client pb.StrategyServiceClient
}

func NewWorkerClient(addr string) (*WorkerClient, error) {
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &WorkerClient{
		conn:   conn,
		client: pb.NewStrategyServiceClient(conn),
	}, nil
}

func (w *WorkerClient) Close() error {
	if w.conn == nil {
		return nil
	}
	return w.conn.Close()
}

// OnTick forwards market data to the worker and translates the response back into Signal.
func (w *WorkerClient) OnTick(ctx context.Context, symbol string, price float64, indicators map[string]float64) (*Signal, error) {
	req := &pb.TickData{
		Symbol:     symbol,
		Price:      price,
		Indicators: indicators,
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	resp, err := w.client.OnTick(ctx, req)
	if err != nil {
		return nil, err
	}
	return &Signal{
		Action: resp.Action,
		Symbol: resp.Symbol,
		Size:   resp.Size,
		Note:   resp.Note,
	}, nil
}
