package proxy

import (
	"context"
	"time"

	"github.com/figment-networks/polkadothub-proxy/grpc/account/accountpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/block/blockpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/chain/chainpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/decode/decodepb"
	"github.com/figment-networks/polkadothub-proxy/grpc/event/eventpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/transaction/transactionpb"
	"golang.org/x/time/rate"
	"google.golang.org/grpc"

	"github.com/pkg/errors"
	"go.uber.org/zap"
)

// ClientIface interface
type ClientIface interface {
	GetAccountBalance(ctx context.Context, account string, height uint64) (*accountpb.GetByHeightResponse, error)
	GetBlockByHeight(ctx context.Context, height uint64) (*blockpb.GetByHeightResponse, error)
	GetMetaByHeight(ctx context.Context, height uint64) (*chainpb.GetMetaByHeightResponse, error)
	GetHead(ctx context.Context) (*chainpb.GetHeadResponse, error)
	GetEventsByHeight(ctx context.Context, height uint64) (*eventpb.GetByHeightResponse, error)
	GetTransactionsByHeight(ctx context.Context, height uint64) (*transactionpb.GetByHeightResponse, error)

	DecodeData(ctx context.Context, block, events, storage []byte) (*decodepb.DecodeResponse, error)
}

// Client connecting to polkadot-proxy
type Client struct {
	log *zap.Logger

	rateLimiter *rate.Limiter

	accountClient     accountpb.AccountServiceClient
	blockClient       blockpb.BlockServiceClient
	chainClient       chainpb.ChainServiceClient
	eventClient       eventpb.EventServiceClient
	transactionClient transactionpb.TransactionServiceClient
	decodeClient      decodepb.DecodeServiceClient
}

// NewClient is a polkadot-proxy Client constructor
func NewClient(log *zap.Logger, rl *rate.Limiter, ac accountpb.AccountServiceClient, bc blockpb.BlockServiceClient,
	cc chainpb.ChainServiceClient, ec eventpb.EventServiceClient, tc transactionpb.TransactionServiceClient, dc decodepb.DecodeServiceClient) *Client {
	return &Client{log: log, rateLimiter: rl, accountClient: ac, blockClient: bc, chainClient: cc, eventClient: ec, transactionClient: tc, decodeClient: dc}
}

// GetAccountBalance return Account Balance by provided height
func (c *Client) DecodeData(ctx context.Context, block, events, storage []byte) (*decodepb.DecodeResponse, error) {

	/*	err := c.rateLimiter.Wait(ctx)
		if err != nil {
			return nil, err
		}
	*/

	now := time.Now()
	res, err := c.decodeClient.Decode(ctx, &decodepb.DecodeRequest{Block: block})
	if err != nil {
		err = errors.Wrapf(err, " error decoding calls : %d")
		rawRequestGRPCDuration.WithLabels("DecodeServiceClient", err.Error()).Observe(time.Since(now).Seconds())
		return nil, err
	}

	rawRequestGRPCDuration.WithLabels("DecodeServiceClient", "OK").Observe(time.Since(now).Seconds())

	return res, err
}

// GetAccountBalance return Account Balance by provided height
func (c *Client) GetAccountBalance(ctx context.Context, account string, height uint64) (*accountpb.GetByHeightResponse, error) {
	c.log.Debug("Sending GetAccountBalanceByHeight height", zap.Uint64("height", height))

	err := c.rateLimiter.Wait(ctx)
	if err != nil {
		return nil, err
	}

	now := time.Now()

	res, err := c.accountClient.GetByHeight(ctx, &accountpb.GetByHeightRequest{Height: int64(height), Address: account})
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
	c.log.Debug("Sending GetBlockByHeight height", zap.Uint64("height", height))

	err := c.rateLimiter.Wait(ctx)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	res, err := c.blockClient.GetByHeight(ctx, &blockpb.GetByHeightRequest{Height: int64(height)}, grpc.WaitForReady(true))
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
	c.log.Debug("Sending GetEventsByHeight height", zap.Uint64("height", height))

	err := c.rateLimiter.Wait(ctx)
	if err != nil {
		return nil, err
	}

	now := time.Now()

	res, err := c.eventClient.GetByHeight(ctx, &eventpb.GetByHeightRequest{Height: int64(height)}, grpc.WaitForReady(true))
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
	c.log.Debug("Sending GetMetaByHeight height", zap.Uint64("height", height))

	err := c.rateLimiter.Wait(ctx)
	if err != nil {
		return nil, err
	}

	now := time.Now()

	res, err := c.chainClient.GetMetaByHeight(ctx, &chainpb.GetMetaByHeightRequest{Height: int64(height)})
	if err != nil {
		err = errors.Wrapf(err, "Error while getting meta by height: %d", height)
		rawRequestGRPCDuration.WithLabels("GetMetaByHeight", err.Error()).Observe(time.Since(now).Seconds())
		return nil, err
	}

	rawRequestGRPCDuration.WithLabels("GetMetaByHeight", "OK").Observe(time.Since(now).Seconds())

	return res, err
}

// GetHead returns Chain meta by height
func (c *Client) GetHead(ctx context.Context) (*chainpb.GetHeadResponse, error) {
	c.log.Debug("Sending GetHead")

	now := time.Now()

	res, err := c.chainClient.GetHead(ctx, &chainpb.GetHeadRequest{})
	if err != nil {
		err = errors.Wrapf(err, "Error while getting head")
		rawRequestGRPCDuration.WithLabels("GetHead", err.Error()).Observe(time.Since(now).Seconds())
		return nil, err
	}

	rawRequestGRPCDuration.WithLabels("GetHead", "OK").Observe(time.Since(now).Seconds())

	return res, err
}

// GetTransactionsByHeight returns Transaction by height
func (c *Client) GetTransactionsByHeight(ctx context.Context, height uint64) (*transactionpb.GetByHeightResponse, error) {
	req := &transactionpb.GetByHeightRequest{
		Height: int64(height),
	}

	err := c.rateLimiter.Wait(ctx)
	if err != nil {
		return nil, err
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
