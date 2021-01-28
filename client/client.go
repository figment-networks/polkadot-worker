package client

import (
	"context"

	"github.com/figment-networks/polkadothub-proxy/grpc/block/blockpb"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// Client connecting to polkadot-proxy
type Client struct {
	Log     *zap.SugaredLogger
	GrcpCli *grpc.ClientConn

	BlockClient blockpb.BlockServiceClient
}

// GetBlockByHeight return Block by provided height
func (c *Client) GetBlockByHeight(height int64) (*blockpb.GetByHeightResponse, error) {
	res, err := c.BlockClient.GetByHeight(context.Background(), &blockpb.GetByHeightRequest{Height: height})
	if err != nil {
		return nil, errors.Wrapf(err, "Error while getting block by height")
	}

	return res, err
}
