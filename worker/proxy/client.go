package proxy

import (
	"context"
<<<<<<< HEAD
	"time"
=======
>>>>>>> Indexer-manager connection

	"github.com/figment-networks/polkadothub-proxy/grpc/block/blockpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/event/eventpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/transaction/transactionpb"

	"github.com/pkg/errors"
	"go.uber.org/zap"
)

// ClientIface interface
type ClientIface interface {
<<<<<<< HEAD
	GetBlockByHeight(ctx context.Context, height uint64) (*blockpb.GetByHeightResponse, error)
	GetEventByHeight(ctx context.Context, height uint64) (*eventpb.GetByHeightResponse, error)
	GetTransactionByHeight(ctx context.Context, height uint64) (*transactionpb.GetByHeightResponse, error)
=======
	GetBlockByHeight(height uint64) (*blockpb.GetByHeightResponse, error)
	GetEventByHeight(height uint64) (*eventpb.GetByHeightResponse, error)
	GetTransactionByHeight(height uint64) (*transactionpb.GetByHeightResponse, error)
>>>>>>> Indexer-manager connection
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
	initMetrics()
	return &Client{log: log, blockClient: bc, eventClient: ec, transactionClient: tc}
}

func initMetrics() {
	BlockConversionDuration = conversionDuration.WithLabels("block")
	TransactionConversionDuration = conversionDuration.WithLabels("transaction")
}

// GetBlockByHeight returns Block by provided height
func (c *Client) GetBlockByHeight(ctx context.Context, height uint64) (*blockpb.GetByHeightResponse, error) {
	req := &blockpb.GetByHeightRequest{
		Height: int64(height),
	}

	c.log.Debugf("Sending GetBlockByHeight height: %d", height)

	now := time.Now()

	res, err := c.blockClient.GetByHeight(ctx, req)
	if err != nil {
		err = errors.Wrapf(err, "Error while getting block by height: %d", height)
		requestDuration.WithLabels("GetBlockByHeight", err.Error()).Observe(time.Since(now).Seconds())
		return nil, err
	}

	requestDuration.WithLabels("GetBlockByHeight", "OK").Observe(time.Since(now).Seconds())

	return res, err
}

// GetEventByHeight returns Event by height
func (c *Client) GetEventByHeight(ctx context.Context, height uint64) (*eventpb.GetByHeightResponse, error) {
	req := &eventpb.GetByHeightRequest{
		Height: int64(height),
	}

	c.log.Debugf("Sending GetEventByHeight height: %d", height)

	now := time.Now()

	res, err := c.eventClient.GetByHeight(ctx, req)
	if err != nil {
		err = errors.Wrapf(err, "Error while getting event by height: %d", height)
		requestDuration.WithLabels("GetEventByHeight", err.Error()).Observe(time.Since(now).Seconds())
		return nil, err
	}

	requestDuration.WithLabels("GetEventByHeight", "OK").Observe(time.Since(now).Seconds())

	return res, err
}

// GetTransactionByHeight returns Transaction by height
func (c *Client) GetTransactionByHeight(ctx context.Context, height uint64) (*transactionpb.GetByHeightResponse, error) {
	req := &transactionpb.GetByHeightRequest{
		Height: int64(height),
	}

	c.log.Debugf("Sending GetTransactionByHeight height: %d", height)

	now := time.Now()

	res, err := c.transactionClient.GetByHeight(ctx, req)
	if err != nil {
		err = errors.Wrapf(err, "Error while getting transaction by height: %d", height)
		requestDuration.WithLabels("GetTransactionByHeight", err.Error()).Observe(time.Since(now).Seconds())
		return nil, err
	}

	requestDuration.WithLabels("GetTransactionByHeight", "OK").Observe(time.Since(now).Seconds())

	return res, err
}
