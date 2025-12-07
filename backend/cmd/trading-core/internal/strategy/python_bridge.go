package strategy

import (
	"context"
	"encoding/json"
	"log"
)

// PythonStrategy forwards ticks to a Python worker via gRPC.
type PythonStrategy struct {
	id     string
	name   string
	client *WorkerClient
}

func NewPythonStrategy(id, name string, client *WorkerClient) *PythonStrategy {
	return &PythonStrategy{
		id:     id,
		name:   name,
		client: client,
	}
}

func (p *PythonStrategy) ID() string { return p.id }

func (p *PythonStrategy) Name() string { return p.name }

func (p *PythonStrategy) GetState() (json.RawMessage, error) {
	return nil, nil // Python strategy state is managed externally
}

func (p *PythonStrategy) SetState(data json.RawMessage) error {
	return nil // No-op
}

func (p *PythonStrategy) OnTick(symbol string, price float64, ind map[string]float64) (*Signal, error) {
	if p.client == nil {
		return nil, nil
	}
	resp, err := p.client.OnTick(context.Background(), symbol, price, ind)
	if err != nil {
		log.Printf("python worker call failed: %v", err)
		return nil, err
	}
	return resp, nil
}
