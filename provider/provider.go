package provider

import (
	"context"

	"github.com/AlhasanIQ/consult-human/contract"
)

type Provider interface {
	Name() string
	Send(ctx context.Context, req contract.AskRequest) (string, error)
	Receive(ctx context.Context, requestID string) (contract.Reply, error)
	Close() error
}
