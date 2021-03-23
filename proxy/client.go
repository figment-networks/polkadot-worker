package proxy

import (
	"context"
	"time"

	"github.com/figment-networks/polkadothub-proxy/grpc/account/accountpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/block/blockpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/chain/chainpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/event/eventpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/transaction/transactionpb"

	"github.com/pkg/errors"
	"go.uber.org/zap"
)

// ClientIface interface
type ClientIface interface {
	GetAccountBalance(ctx context.Context, account string, height uint64) (*accountpb.GetByHeightResponse, error)
	GetBlockByHeight(ctx context.Context, height uint64) (*blockpb.GetByHeightResponse, error)
	GetMetaByHeight(ctx context.Context, height uint64) (*chainpb.GetMetaByHeightResponse, error)
	GetEventsByHeight(ctx context.Context, height uint64) (*eventpb.GetByHeightResponse, error)
	GetTransactionsByHeight(ctx context.Context, height uint64) (*transactionpb.GetByHeightResponse, error)
}

// Client connecting to polkadot-proxy
type Client struct {
	log *zap.Logger

	accountClient     accountpb.AccountServiceClient
	blockClient       blockpb.BlockServiceClient
	chainClient       chainpb.ChainServiceClient
	eventClient       eventpb.EventServiceClient
	transactionClient transactionpb.TransactionServiceClient
}

// NewClient is a polkadot-proxy Client constructor
func NewClient(log *zap.Logger, ac accountpb.AccountServiceClient, bc blockpb.BlockServiceClient,
	cc chainpb.ChainServiceClient, ec eventpb.EventServiceClient, tc transactionpb.TransactionServiceClient) *Client {
	return &Client{log: log, accountClient: ac, blockClient: bc, chainClient: cc, eventClient: ec, transactionClient: tc}
}

// GetAccountBalance return Account Balance by provided height
func (c *Client) GetAccountBalance(ctx context.Context, account string, height uint64) (*accountpb.GetByHeightResponse, error) {
	req := &accountpb.GetByHeightRequest{
		Height:  int64(height),
		Address: account,
	}

	c.log.Debug("Sending GetAccountBalanceByHeight height", zap.Uint64("height", height))

	now := time.Now()

	res, err := c.accountClient.GetByHeight(ctx, req)
	if err != nil {
		err = errors.Wrapf(err, "Error while getting account balance by height: %d", height)
		rawRequestGRPCDuration.WithLabels("GetAccountBalanceByHeight", err.Error()).Observe(time.Since(now).Seconds())
		return nil, err
	}

	rawRequestGRPCDuration.WithLabels("GetAccountBalanceByHeight", "OK").Observe(time.Since(now).Seconds())

	return res, err
}

// GetBlockByHeight returns Block by provided height
func (c *Client) GetBlockByHeight(ctx context.Context, height uint64) (*blockpb.GetByHeightResponse, error) {
	req := &blockpb.GetByHeightRequest{
		Height: int64(height),
	}

	c.log.Debug("Sending GetBlockByHeight height", zap.Uint64("height", height))

	now := time.Now()

	res, err := c.blockClient.GetByHeight(ctx, req)
	if err != nil {
		err = errors.Wrapf(err, "Error while getting block by height: %d", height)
		rawRequestGRPCDuration.WithLabels("GetBlockByHeight", err.Error()).Observe(time.Since(now).Seconds())
		return nil, err
	}

	rawRequestGRPCDuration.WithLabels("GetBlockByHeight", "OK").Observe(time.Since(now).Seconds())

	return res, err
}

// GetEventsByHeight returns Event by height
func (c *Client) GetEventsByHeight(ctx context.Context, height uint64) (*eventpb.GetByHeightResponse, error) {
	req := &eventpb.GetByHeightRequest{
		Height: int64(height),
	}

	c.log.Debug("Sending GetEventsByHeight height", zap.Uint64("height", height))

	now := time.Now()

	res, err := c.eventClient.GetByHeight(ctx, req)
	if err != nil {
		err = errors.Wrapf(err, "Error while getting event by height: %d", height)
		rawRequestGRPCDuration.WithLabels("GetEventsByHeight", err.Error()).Observe(time.Since(now).Seconds())
		return nil, err
	}

	rawRequestGRPCDuration.WithLabels("GetEventsByHeight", "OK").Observe(time.Since(now).Seconds())

	return res, err
}

// GetMetaByHeight returns Chain meta by height
func (c *Client) GetMetaByHeight(ctx context.Context, height uint64) (*chainpb.GetMetaByHeightResponse, error) {
	req := &chainpb.GetMetaByHeightRequest{
		Height: int64(height),
	}

	c.log.Debug("Sending GetMetaByHeight height", zap.Uint64("height", height))

	now := time.Now()

	res, err := c.chainClient.GetMetaByHeight(ctx, req)
	if err != nil {
		err = errors.Wrapf(err, "Error while getting meta by height: %d", height)
		rawRequestGRPCDuration.WithLabels("GetMetaByHeight", err.Error()).Observe(time.Since(now).Seconds())
		return nil, err
	}

	rawRequestGRPCDuration.WithLabels("GetMetaByHeight", "OK").Observe(time.Since(now).Seconds())

	return res, err
}

// GetTransactionsByHeight returns Transaction by height
func (c *Client) GetTransactionsByHeight(ctx context.Context, height uint64) (*transactionpb.GetByHeightResponse, error) {
	req := &transactionpb.GetByHeightRequest{
		Height: int64(height),
	}

	c.log.Debug("Sending GetTransactionsByHeight height", zap.Uint64("height", height))

	now := time.Now()

	res, err := c.transactionClient.GetByHeight(ctx, req)
	if err != nil {
		err = errors.Wrapf(err, "Error while getting transaction by height: %d", height)
		rawRequestGRPCDuration.WithLabels("GetTransactionsByHeight", err.Error()).Observe(time.Since(now).Seconds())
		return nil, err
	}

	rawRequestGRPCDuration.WithLabels("GetTransactionsByHeight", "OK").Observe(time.Since(now).Seconds())

	return res, err
}
