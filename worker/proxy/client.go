package proxy

import (
	"context"

	"github.com/figment-networks/polkadothub-proxy/grpc/block/blockpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/event/eventpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/transaction/transactionpb"

	"github.com/pkg/errors"
	"go.uber.org/zap"
)

// ClientIface interface
type ClientIface interface {
	GetBlockByHeight(ctx context.Context, height uint64) (*blockpb.GetByHeightResponse, error)
	GetEventByHeight(ctx context.Context, height uint64) (*eventpb.GetByHeightResponse, error)
	GetTransactionByHeight(ctx context.Context, height uint64) (*transactionpb.GetByHeightResponse, error)
}

// Client connecting to polkadot-proxy
type Client struct {
	log *zap.SugaredLogger

	blockClient       blockpb.BlockServiceClient
	eventClient       eventpb.EventServiceClient
	transactionClient transactionpb.TransactionServiceClient
}

// NewClient is a polkadot-proxy Client constructor
func NewClient(log *zap.SugaredLogger, bc blockpb.BlockServiceClient, ec eventpb.EventServiceClient, tc transactionpb.TransactionServiceClient) *Client {
	return &Client{log: log, blockClient: bc, eventClient: ec, transactionClient: tc}
}

// GetBlockByHeight returns Block by provided height
func (c *Client) GetBlockByHeight(ctx context.Context, height uint64) (*blockpb.GetByHeightResponse, error) {
	req := &blockpb.GetByHeightRequest{
		Height: int64(height),
	}

	c.log.Debugf("Sending GetBlockByHeight height: %d", height)

	res, err := c.blockClient.GetByHeight(ctx, req)
	if err != nil {
		return nil, errors.Wrapf(err, "Error while getting block by height: %d", height)
	}

	return res, err
}

// GetEventByHeight returns Event by height
func (c *Client) GetEventByHeight(ctx context.Context, height uint64) (*eventpb.GetByHeightResponse, error) {
	req := &eventpb.GetByHeightRequest{
		Height: int64(height),
	}

	c.log.Debugf("Sending GetEventByHeight height: %d", height)

	res, err := c.eventClient.GetByHeight(ctx, req)
	if err != nil {
		return nil, errors.Wrapf(err, "Error while getting event by height: %d", height)
	}

	return res, err
}

// GetTransactionByHeight returns Transaction by height
func (c *Client) GetTransactionByHeight(ctx context.Context, height uint64) (*transactionpb.GetByHeightResponse, error) {
	req := &transactionpb.GetByHeightRequest{
		Height: int64(height),
	}

	c.log.Debugf("Sending GetTransactionByHeight height: %d", height)

	res, err := c.transactionClient.GetByHeight(ctx, req)
	if err != nil {
		return nil, errors.Wrapf(err, "Error while getting transaction by height: %d", height)
	}

	return res, err
}
