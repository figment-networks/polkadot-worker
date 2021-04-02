package proxy

import (
	"context"
	"time"

	"github.com/figment-networks/polkadot-worker/structs"

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

// Client connecting to polkadot-proxy
type Client struct {
	log *zap.Logger

	rateLimiter *rate.Limiter

	AccountClient     accountpb.AccountServiceClient
	BlockClient       blockpb.BlockServiceClient
	ChainClient       chainpb.ChainServiceClient
	EventClient       eventpb.EventServiceClient
	TransactionClient transactionpb.TransactionServiceClient
	DecodeClient      decodepb.DecodeServiceClient
}

// NewClient is a polkadot-proxy Client constructor
func NewClient(log *zap.Logger, rl *rate.Limiter, conn *grpc.ClientConn) *Client {
	return &Client{log: log,
		rateLimiter:       rl,
		AccountClient:     accountpb.NewAccountServiceClient(conn),
		BlockClient:       blockpb.NewBlockServiceClient(conn),
		ChainClient:       chainpb.NewChainServiceClient(conn),
		EventClient:       eventpb.NewEventServiceClient(conn),
		TransactionClient: transactionpb.NewTransactionServiceClient(conn),
		DecodeClient:      decodepb.NewDecodeServiceClient(conn)}
}

// GetAccountBalance return Account Balance by provided height
func (c *Client) DecodeData(ctx context.Context, ddr structs.DecodeDataRequest) (*decodepb.DecodeResponse, error) {
	now := time.Now()
	res, err := c.DecodeClient.Decode(ctx, &decodepb.DecodeRequest{
		MetadataParent:          ddr.MetadataParent,
		Block:                   ddr.Block,
		BlockHash:               ddr.BlockHash,
		Events:                  ddr.Events,
		Timestamp:               ddr.Timestamp,
		RuntimeParent:           ddr.RuntimeParent,
		CurrentEraParent:        ddr.CurrentEra,
		NextFeeMultiplierParent: ddr.NextFeeMultipier,
		Chain:                   ddr.Chain,
	})
	if err != nil {
		rawRequestGRPCDuration.WithLabels("DecodeServiceClient", "ERR").Observe(time.Since(now).Seconds())
		return nil, errors.Wrapf(err, "error calling decode")
	}

	rawRequestGRPCDuration.WithLabels("DecodeServiceClient", "OK").Observe(time.Since(now).Seconds())

	return res, err
}

// GetAccountBalance return Account Balance by provided height
func (c *Client) GetAccountBalance(ctx context.Context, account string, height uint64) (*accountpb.GetByHeightResponse, error) {
	c.log.Debug("Sending GetAccountBalanceByHeight", zap.Uint64("height", height))

	err := c.rateLimiter.Wait(ctx)
	if err != nil {
		return nil, err
	}

	now := time.Now()

	res, err := c.AccountClient.GetByHeight(ctx, &accountpb.GetByHeightRequest{Height: int64(height), Address: account})
	if err != nil {
		err = errors.Wrapf(err, "Error while getting account balance by height: %d", height)
		rawRequestGRPCDuration.WithLabels("GetAccountBalanceByHeight", "ERR").Observe(time.Since(now).Seconds())
		return nil, err
	}

	took := time.Since(now).Seconds()
	c.log.Debug("Received GetAccountBalanceByHeight", zap.Float64("took", took))

	rawRequestGRPCDuration.WithLabels("GetAccountBalanceByHeight", "OK").Observe(took)

	return res, err
}

// GetBlockByHeight returns Block by provided height
func (c *Client) GetBlockByHeight(ctx context.Context, height uint64) (*blockpb.GetByHeightResponse, error) {
	c.log.Debug("Sending GetBlockByHeight", zap.Uint64("height", height))

	err := c.rateLimiter.Wait(ctx)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	res, err := c.BlockClient.GetByHeight(ctx, &blockpb.GetByHeightRequest{Height: int64(height)}, grpc.WaitForReady(true))
	if err != nil {
		err = errors.Wrapf(err, "Error while getting block by height: %d", height)
		rawRequestGRPCDuration.WithLabels("GetBlockByHeight", "ERR").Observe(time.Since(now).Seconds())
		return nil, err
	}

	took := time.Since(now).Seconds()
	c.log.Debug("Received GetBlockByHeight", zap.Float64("took", took))

	rawRequestGRPCDuration.WithLabels("GetBlockByHeight", "OK").Observe(took)

	return res, err
}

// GetEventsByHeight returns Event by height
func (c *Client) GetEventsByHeight(ctx context.Context, height uint64) (*eventpb.GetByHeightResponse, error) {
	c.log.Debug("Sending GetEventsByHeight", zap.Uint64("height", height))

	err := c.rateLimiter.Wait(ctx)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	res, err := c.EventClient.GetByHeight(ctx, &eventpb.GetByHeightRequest{Height: int64(height)}, grpc.WaitForReady(true))
	if err != nil {
		err = errors.Wrapf(err, "Error while getting event by height: %d", height)
		rawRequestGRPCDuration.WithLabels("GetEventsByHeight", "ERR").Observe(time.Since(now).Seconds())
		return nil, err
	}

	took := time.Since(now).Seconds()
	c.log.Debug("Received GetEventsByHeight", zap.Float64("took", took))

	rawRequestGRPCDuration.WithLabels("GetEventsByHeight", "OK").Observe(took)

	return res, err
}

// GetMetaByHeight returns Chain meta by height
func (c *Client) GetMetaByHeight(ctx context.Context, height uint64) (*chainpb.GetMetaByHeightResponse, error) {
	c.log.Debug("Sending GetMetaByHeight", zap.Uint64("height", height))

	err := c.rateLimiter.Wait(ctx)
	if err != nil {
		return nil, err
	}

	now := time.Now()

	res, err := c.ChainClient.GetMetaByHeight(ctx, &chainpb.GetMetaByHeightRequest{Height: int64(height)})
	if err != nil {
		err = errors.Wrapf(err, "Error while getting meta by height: %d", height)
		rawRequestGRPCDuration.WithLabels("GetMetaByHeight", "ERR").Observe(time.Since(now).Seconds())
		return nil, err
	}

	took := time.Since(now).Seconds()
	c.log.Debug("Received GetEventsByHeight", zap.Float64("took", took))

	rawRequestGRPCDuration.WithLabels("GetMetaByHeight", "OK").Observe(took)

	return res, err
}

// GetHead returns Chain meta by height
func (c *Client) GetHead(ctx context.Context) (*chainpb.GetHeadResponse, error) {
	c.log.Debug("Sending GetHead")

	now := time.Now()

	res, err := c.ChainClient.GetHead(ctx, &chainpb.GetHeadRequest{})
	if err != nil {
		err = errors.Wrapf(err, "Error while getting head")
		rawRequestGRPCDuration.WithLabels("GetHead", "ERR").Observe(time.Since(now).Seconds())
		return nil, err
	}

	took := time.Since(now).Seconds()
	c.log.Debug("Received GetEventsByHeight", zap.Float64("took", took))

	rawRequestGRPCDuration.WithLabels("GetHead", "OK").Observe(took)

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

	c.log.Debug("Sending GetTransactionsByHeight", zap.Uint64("height", height))

	now := time.Now()

	res, err := c.TransactionClient.GetByHeight(ctx, req)
	if err != nil {
		err = errors.Wrapf(err, "Error while getting transaction by height: %d", height)
		rawRequestGRPCDuration.WithLabels("GetTransactionsByHeight", err.Error()).Observe(time.Since(now).Seconds())
		return nil, err
	}

	took := time.Since(now).Seconds()
	c.log.Debug("Received GetEventsByHeight", zap.Float64("took", took))

	rawRequestGRPCDuration.WithLabels("GetTransactionsByHeight", "OK").Observe(took)

	return res, err
}
