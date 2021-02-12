package proxy

import (
	"context"
	"time"

	"github.com/figment-networks/polkadothub-proxy/grpc/block/blockpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/event/eventpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/transaction/transactionpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/validator/validatorpb"

	"github.com/pkg/errors"
	"go.uber.org/zap"
)

// ClientIface interface
type ClientIface interface {
	GetBlockByHeight(ctx context.Context, height uint64) (*blockpb.GetByHeightResponse, error)
	GetEventsByHeight(ctx context.Context, height uint64) (*eventpb.GetByHeightResponse, error)
	GetTransactionsByHeight(ctx context.Context, height uint64) (*transactionpb.GetByHeightResponse, error)
	GetValidatorsByHeight(ctx context.Context, height uint64) (*validatorpb.GetAllByHeightResponse, error)
}

// Client connecting to polkadot-proxy
type Client struct {
	log *zap.SugaredLogger

	blockClient       blockpb.BlockServiceClient
	eventClient       eventpb.EventServiceClient
	transactionClient transactionpb.TransactionServiceClient
	validatorClient   validatorpb.ValidatorServiceClient
}

// NewClient is a polkadot-proxy Client constructor
func NewClient(log *zap.SugaredLogger, bc blockpb.BlockServiceClient, ec eventpb.EventServiceClient,
	tc transactionpb.TransactionServiceClient, vc validatorpb.ValidatorServiceClient) *Client {
	initMetrics()
	return &Client{log: log, blockClient: bc, eventClient: ec, transactionClient: tc, validatorClient: vc}
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

// GetEventsByHeight returns Event by height
func (c *Client) GetEventsByHeight(ctx context.Context, height uint64) (*eventpb.GetByHeightResponse, error) {
	req := &eventpb.GetByHeightRequest{
		Height: int64(height),
	}

	c.log.Debugf("Sending GetEventsByHeight height: %d", height)

	now := time.Now()

	res, err := c.eventClient.GetByHeight(ctx, req)
	if err != nil {
		err = errors.Wrapf(err, "Error while getting event by height: %d", height)
		requestDuration.WithLabels("GetEventsByHeight", err.Error()).Observe(time.Since(now).Seconds())
		return nil, err
	}

	requestDuration.WithLabels("GetEventsByHeight", "OK").Observe(time.Since(now).Seconds())

	return res, err
}

// GetTransactionsByHeight returns Transaction by height
func (c *Client) GetTransactionsByHeight(ctx context.Context, height uint64) (*transactionpb.GetByHeightResponse, error) {
	req := &transactionpb.GetByHeightRequest{
		Height: int64(height),
	}

	c.log.Debugf("Sending GetTransactionsByHeight height: %d", height)

	now := time.Now()

	res, err := c.transactionClient.GetByHeight(ctx, req)
	if err != nil {
		err = errors.Wrapf(err, "Error while getting transaction by height: %d", height)
		requestDuration.WithLabels("GetTransactionsByHeight", err.Error()).Observe(time.Since(now).Seconds())
		return nil, err
	}

	requestDuration.WithLabels("GetTransactionsByHeight", "OK").Observe(time.Since(now).Seconds())

	return res, err
}

// GetValidatorsByHeight returns Validators by height
func (c *Client) GetValidatorsByHeight(ctx context.Context, height uint64) (*validatorpb.GetAllByHeightResponse, error) {
	req := &validatorpb.GetAllByHeightRequest{
		Height: int64(height),
	}

	c.log.Debugf("Sending GetValidatorsByHeight height: %d", height)

	now := time.Now()

	res, err := c.validatorClient.GetAllByHeight(ctx, req)
	if err != nil {
		err = errors.Wrapf(err, "Error while getting validators by height: %d", height)
		requestDuration.WithLabels("GetValidatorsByHeight", err.Error()).Observe(time.Since(now).Seconds())
		return nil, err
	}

	requestDuration.WithLabels("GetValidatorsByHeight", "OK").Observe(time.Since(now).Seconds())

	return res, err
}
