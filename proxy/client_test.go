package proxy_test

import (
	"context"
	"errors"
	"testing"

	"github.com/figment-networks/polkadot-worker/proxy"

	"github.com/figment-networks/polkadothub-proxy/grpc/account/accountpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/block/blockpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/chain/chainpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/decode/decodepb"
	"github.com/figment-networks/polkadothub-proxy/grpc/event/eventpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/transaction/transactionpb"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
	"google.golang.org/grpc"
)

type ClientTest struct {
	suite.Suite

	*connMock
	*proxy.Client
}

func (c *ClientTest) SetupTest() {
	logger, err := zap.NewDevelopment()
	c.Require().Nil(err)

	rateLimiter := rate.NewLimiter(rate.Limit(100), 100)

	c.connMock = &connMock{
		AccountClientMock:     &accountClientMock{},
		BlockClientMock:       &blockClientMock{},
		ChainClientMock:       &chainClientMock{},
		DecodeClientMock:      &decodeClientMock{},
		EventClientMock:       &eventClientMock{},
		TransactionClientMock: &transactionClientMock{},
	}

	c.Client = proxy.NewClient(
		logger,
		rateLimiter,
		c.connMock,
	)
}

func (c *ClientTest) TestGetBlockByHeight_OK() {
	height := int64(120)

	req := &blockpb.GetByHeightRequest{
		Height: height,
	}

	res := &blockpb.GetByHeightResponse{
		Block: &blockpb.Block{
			BlockHash: "0x2326841a64e0a3fff2b4bb760d316cc74b33a8a9480a28ab7e7885acba85e3cf",
			Header: &blockpb.Header{
				Height: height,
			},
			Extrinsics: []*transactionpb.Transaction{
				{
					ExtrinsicIndex: 2,
					Hash:           "0x2326841a64e0a3fff2b4bb760d316cc74b33a8a9480a28ab7e7885acba85e3cf",
					Events: []*eventpb.Event{
						{
							Index:          1,
							ExtrinsicIndex: 2,
						},
					},
				},
			},
		},
	}

	c.BlockClientMock.On("GetByHeight", mock.AnythingOfType("*context.emptyCtx"), req, mock.AnythingOfType("[]grpc.CallOption")).Return(res, nil)

	response, err := c.GetBlockByHeight(context.Background(), uint64(height))

	c.Require().Nil(err)

	c.Require().NotNil(response)
	c.Require().Equal(res.Block.BlockHash, response.Block.BlockHash)
	c.Require().EqualValues(height, response.Block.Header.Height)

	transactions := response.Block.Extrinsics
	c.Require().Len(transactions, 1)
	c.Require().Equal(transactions[0].ExtrinsicIndex, response.Block.Extrinsics[0].ExtrinsicIndex)
	c.Require().Equal(transactions[0].Hash, response.Block.Extrinsics[0].Hash)

	events := transactions[0].Events
	c.Require().Len(events, 1)
	c.Require().Equal(events[0].Index, res.Block.Extrinsics[0].Events[0].Index)
	c.Require().Equal(events[0].ExtrinsicIndex, res.Block.Extrinsics[0].Events[0].ExtrinsicIndex)
}

func (c *ClientTest) TestGetBlockByHeight_Error() {
	height := int64(120)

	req := &blockpb.GetByHeightRequest{
		Height: height,
	}
	res := &blockpb.GetByHeightResponse{}

	e := errors.New("new polkadothub-proxy error")

	c.BlockClientMock.On("GetByHeight", mock.AnythingOfType("*context.emptyCtx"), req, mock.AnythingOfType("[]grpc.CallOption")).Return(res, e)

	response, err := c.GetBlockByHeight(context.Background(), uint64(height))

	c.Require().Nil(response)

	c.Require().Contains(err.Error(), "Error while getting block by height: 120: new polkadothub-proxy error")
}

func (c *ClientTest) TestGetEventByHeight_OK() {
	height := int64(120)

	req := &eventpb.GetByHeightRequest{
		Height: height,
	}

	res := &eventpb.GetByHeightResponse{
		Events: []*eventpb.Event{
			{
				Index:          1,
				ExtrinsicIndex: 2,
			},
		},
	}

	c.EventClientMock.On("GetByHeight", mock.AnythingOfType("*context.emptyCtx"), req, mock.AnythingOfType("[]grpc.CallOption")).Return(res, nil)

	response, err := c.GetEventsByHeight(context.Background(), uint64(height))

	c.Require().Nil(err)

	c.Require().Len(response.Events, 1)
	c.Require().Equal(res.Events[0].Index, response.Events[0].Index)
	c.Require().Equal(res.Events[0].ExtrinsicIndex, response.Events[0].ExtrinsicIndex)
}

func (c *ClientTest) TestGetEventByHeight_Error() {
	height := int64(120)

	req := &eventpb.GetByHeightRequest{
		Height: height,
	}
	res := &eventpb.GetByHeightResponse{}

	e := errors.New("new polkadothub-proxy error")

	c.EventClientMock.On("GetByHeight", mock.AnythingOfType("*context.emptyCtx"), req, mock.AnythingOfType("[]grpc.CallOption")).Return(res, e)

	response, err := c.GetEventsByHeight(context.Background(), uint64(height))

	c.Require().Nil(response)

	c.Require().Contains(err.Error(), "Error while getting event by height: 120: new polkadothub-proxy error")
}

func (c *ClientTest) TestGetMetaByHeight_OK() {
	height := int64(120)

	req := &chainpb.GetMetaByHeightRequest{
		Height: height,
	}

	res := &chainpb.GetMetaByHeightResponse{
		Era: 123,
	}

	c.ChainClientMock.On("GetMetaByHeight", mock.AnythingOfType("*context.emptyCtx"), req, mock.AnythingOfType("[]grpc.CallOption")).Return(res, nil)

	response, err := c.GetMetaByHeight(context.Background(), uint64(height))

	c.Require().Nil(err)

	c.Require().Equal(res.Era, response.Era)
}

func (c *ClientTest) TestGetMetaByHeight_Error() {
	height := int64(120)

	req := &chainpb.GetMetaByHeightRequest{
		Height: height,
	}

	e := errors.New("new polkadothub-proxy error")

	c.ChainClientMock.On("GetMetaByHeight", mock.AnythingOfType("*context.emptyCtx"), req, mock.AnythingOfType("[]grpc.CallOption")).Return(&chainpb.GetMetaByHeightResponse{}, e)

	response, err := c.GetMetaByHeight(context.Background(), uint64(height))

	c.Require().Nil(response)

	c.Require().Contains(err.Error(), "Error while getting meta by height: 120: new polkadothub-proxy error")
}

func (c *ClientTest) TestGetTransactionByHeight_OK() {
	height := int64(120)

	req := &transactionpb.GetByHeightRequest{
		Height: height,
	}

	res := &transactionpb.GetByHeightResponse{
		Transactions: []*transactionpb.Transaction{
			{
				ExtrinsicIndex: 2,
				Hash:           "0x2326841a64e0a3fff2b4bb760d316cc74b33a8a9480a28ab7e7885acba85e3cf",
				Events: []*eventpb.Event{
					{
						Index:          1,
						ExtrinsicIndex: 2,
					},
				},
			},
		},
	}

	c.TransactionClientMock.On("GetByHeight", mock.AnythingOfType("*context.emptyCtx"), req, mock.AnythingOfType("[]grpc.CallOption")).Return(res, nil)

	response, err := c.GetTransactionsByHeight(context.Background(), uint64(height))

	c.Require().Nil(err)

	c.Require().Len(response.Transactions, 1)
	c.Require().Equal(res.Transactions[0].ExtrinsicIndex, response.Transactions[0].ExtrinsicIndex)
	c.Require().Equal(res.Transactions[0].Hash, response.Transactions[0].Hash)

	events := response.Transactions[0].Events
	c.Require().Len(events, 1)
	c.Require().Equal(events[0].Index, res.Transactions[0].Events[0].Index)
	c.Require().Equal(events[0].ExtrinsicIndex, res.Transactions[0].Events[0].ExtrinsicIndex)
}

func (c *ClientTest) TestGetTransactionByHeight_Error() {
	height := int64(120)

	req := &transactionpb.GetByHeightRequest{
		Height: height,
	}
	res := &transactionpb.GetByHeightResponse{}

	e := errors.New("new polkadothub-proxy error")

	c.TransactionClientMock.On("GetByHeight", mock.AnythingOfType("*context.emptyCtx"), req, mock.AnythingOfType("[]grpc.CallOption")).Return(res, e)

	response, err := c.GetTransactionsByHeight(context.Background(), uint64(height))

	c.Require().Nil(response)

	c.Require().Contains(err.Error(), "Error while getting transaction by height: 120: new polkadothub-proxy error")
}

func TestBlockClient(t *testing.T) {
	suite.Run(t, new(ClientTest))
}

type accountClientMock struct {
	mock.Mock
}

func (m accountClientMock) GetByHeight(ctx context.Context, in *accountpb.GetByHeightRequest, opts ...grpc.CallOption) (*accountpb.GetByHeightResponse, error) {
	args := m.Called(ctx, in, opts)
	return args.Get(0).(*accountpb.GetByHeightResponse), args.Error(1)
}

func (m accountClientMock) GetIdentity(ctx context.Context, in *accountpb.GetIdentityRequest, opts ...grpc.CallOption) (*accountpb.GetIdentityResponse, error) {
	args := m.Called(ctx, in, opts)
	return args.Get(0).(*accountpb.GetIdentityResponse), args.Error(1)
}

type blockClientMock struct {
	mock.Mock
}

func (m blockClientMock) GetByHeight(ctx context.Context, in *blockpb.GetByHeightRequest, opts ...grpc.CallOption) (*blockpb.GetByHeightResponse, error) {
	args := m.Called(ctx, in, opts)
	return args.Get(0).(*blockpb.GetByHeightResponse), args.Error(1)
}

type chainClientMock struct {
	mock.Mock
}

func (m chainClientMock) GetHead(ctx context.Context, in *chainpb.GetHeadRequest, opts ...grpc.CallOption) (*chainpb.GetHeadResponse, error) {
	args := m.Called(ctx, in, opts)
	return args.Get(0).(*chainpb.GetHeadResponse), args.Error(1)
}

func (m chainClientMock) GetStatus(ctx context.Context, in *chainpb.GetStatusRequest, opts ...grpc.CallOption) (*chainpb.GetStatusResponse, error) {
	args := m.Called(ctx, in, opts)
	return args.Get(0).(*chainpb.GetStatusResponse), args.Error(1)
}

func (m chainClientMock) GetMetaByHeight(ctx context.Context, in *chainpb.GetMetaByHeightRequest, opts ...grpc.CallOption) (*chainpb.GetMetaByHeightResponse, error) {
	args := m.Called(ctx, in, opts)
	return args.Get(0).(*chainpb.GetMetaByHeightResponse), args.Error(1)
}

type decodeClientMock struct {
	mock.Mock
}

func (m decodeClientMock) Decode(ctx context.Context, in *decodepb.DecodeRequest, opts ...grpc.CallOption) (*decodepb.DecodeResponse, error) {
	args := m.Called(ctx, in, opts)
	return args.Get(0).(*decodepb.DecodeResponse), args.Error(1)
}

type eventClientMock struct {
	mock.Mock
}

func (m eventClientMock) GetByHeight(ctx context.Context, in *eventpb.GetByHeightRequest, opts ...grpc.CallOption) (*eventpb.GetByHeightResponse, error) {
	args := m.Called(ctx, in, opts)
	return args.Get(0).(*eventpb.GetByHeightResponse), args.Error(1)
}

type transactionClientMock struct {
	mock.Mock
}

func (m transactionClientMock) GetAnnotatedByHeight(ctx context.Context, in *transactionpb.GetAnnotatedByHeightRequest, opts ...grpc.CallOption) (*transactionpb.GetAnnotatedByHeightResponse, error) {
	args := m.Called(ctx, in, opts)
	return args.Get(0).(*transactionpb.GetAnnotatedByHeightResponse), args.Error(1)
}

func (m transactionClientMock) GetByHeight(ctx context.Context, in *transactionpb.GetByHeightRequest, opts ...grpc.CallOption) (*transactionpb.GetByHeightResponse, error) {
	args := m.Called(ctx, in, opts)
	return args.Get(0).(*transactionpb.GetByHeightResponse), args.Error(1)
}

func (m transactionClientMock) GetHead(ctx context.Context) (*chainpb.GetHeadResponse, error) {
	args := m.Called(ctx)
	return args.Get(0).(*chainpb.GetHeadResponse), args.Error(1)
}

type connMock struct {
	mock.Mock

	AccountClientMock     *accountClientMock
	BlockClientMock       *blockClientMock
	ChainClientMock       *chainClientMock
	DecodeClientMock      *decodeClientMock
	EventClientMock       *eventClientMock
	TransactionClientMock *transactionClientMock
}

func (m connMock) Close() {
	return
}

func (m connMock) Add(*grpc.ClientConn) {
	return
}

func (m connMock) GetNextAccountClient() accountpb.AccountServiceClient {
	return m.AccountClientMock
}

func (m connMock) GetNextBlockClient() blockpb.BlockServiceClient {
	return m.BlockClientMock
}

func (m connMock) GetNextChainClient() chainpb.ChainServiceClient {
	return m.ChainClientMock
}

func (m connMock) GetNextEventServiceClient() eventpb.EventServiceClient {
	return m.EventClientMock
}

func (m connMock) GetNextTransactionServiceClient() transactionpb.TransactionServiceClient {
	return m.TransactionClientMock
}

func (m connMock) GetNextDecodeServiceClient() decodepb.DecodeServiceClient {
	return m.DecodeClientMock
}
