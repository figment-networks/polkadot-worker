package worker

import (
	"context"

	"github.com/figment-networks/polkadothub-proxy/grpc/block/blockpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/transaction/transactionpb"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// Client connecting to polkadot-proxy
type Client struct {
	Log     *zap.SugaredLogger
	GrcpCli *grpc.ClientConn

	BlockClient       blockpb.BlockServiceClient
	TransactionClient transactionpb.TransactionServiceClient
}

var errNotFound = errors.New("not found")

// GetBlockByHeight return Block by provided height
func (c *Client) GetBlockByHeight(height int64) (*blockpb.GetByHeightResponse, error) {
	req := &blockpb.GetByHeightRequest{
		Height: height,
	}

	res, err := c.BlockClient.GetByHeight(context.Background(), req)
	if err != nil {
		return nil, errors.Wrapf(err, "Error while getting block by height")
	}
	if res == nil || res.Block == nil {
		return nil, errNotFound
	}

	return res, err
}

// GetTransactionByHeight return Transaction by height
func (c *Client) GetTransactionByHeight(height int64) (*transactionpb.GetByHeightResponse, error) {
	req := &transactionpb.GetByHeightRequest{
		Height: height,
	}

	res, err := c.TransactionClient.GetByHeight(context.Background(), req)
	if err != nil {
		return nil, errors.Wrapf(err, "Error while getting transaction by height")
	}
	if res == nil || res.Transactions == nil {
		return nil, errNotFound
	}

	return res, err
}
