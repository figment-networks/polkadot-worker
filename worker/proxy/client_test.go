package proxy_test

import (
	"context"
	"errors"
	"testing"

	"github.com/figment-networks/polkadot-worker/worker/proxy"

	"github.com/figment-networks/polkadothub-proxy/grpc/block/blockpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/event/eventpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/transaction/transactionpb"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type BlockClientTest struct {
	suite.Suite

	*proxy.Client

	BlockClientMock       *blockClientMock
	EventClientMock       *eventClientMock
	TransactionClientMock *transactionClientMock
}

func (bc *BlockClientTest) SetupTest() {
	logger, err := zap.NewDevelopment()
	bc.Require().Nil(err)

	blockClientMock := blockClientMock{}
	eventClientMock := eventClientMock{}
	transactionClientMock := transactionClientMock{}

	bc.Client = proxy.NewClient(logger.Sugar(), &blockClientMock, &eventClientMock, &transactionClientMock)
	bc.BlockClientMock = &blockClientMock
	bc.EventClientMock = &eventClientMock
	bc.TransactionClientMock = &transactionClientMock
}

func (bc *BlockClientTest) TestGetBlockByHeight_OK() {
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
		},
	}

	bc.BlockClientMock.On("GetByHeight", mock.AnythingOfType("*context.emptyCtx"), req, mock.AnythingOfType("[]grpc.CallOption")).Return(res, nil)

	response, err := bc.GetBlockByHeight(uint64(height))

	bc.Require().Nil(err)

	bc.Require().NotNil(response)
	bc.Require().Equal(res.Block.BlockHash, response.Block.BlockHash)
	bc.Require().EqualValues(height, response.Block.Header.Height)
}

func (bc *BlockClientTest) TestGetBlockByHeight_Error() {
	height := int64(120)

	req := &blockpb.GetByHeightRequest{
		Height: height,
	}
	res := &blockpb.GetByHeightResponse{}

	e := errors.New("new polkadothub-proxy error")

	bc.BlockClientMock.On("GetByHeight", mock.AnythingOfType("*context.emptyCtx"), req, mock.AnythingOfType("[]grpc.CallOption")).Return(res, e)

	response, err := bc.GetBlockByHeight(uint64(height))

	bc.Require().Nil(response)

	bc.Require().Contains(err.Error(), "Error while getting block by height: 120: new polkadothub-proxy error")
}

func (bc *BlockClientTest) TestGetEventByHeight_OK() {
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

	bc.EventClientMock.On("GetByHeight", mock.AnythingOfType("*context.emptyCtx"), req, mock.AnythingOfType("[]grpc.CallOption")).Return(res, nil)

	response, err := bc.GetEventByHeight(uint64(height))

	bc.Require().Nil(err)

	bc.Require().Len(response.Events, 1)
	bc.Require().Equal(res.Events[0].Index, response.Events[0].Index)
	bc.Require().Equal(res.Events[0].ExtrinsicIndex, response.Events[0].ExtrinsicIndex)
}

func (bc *BlockClientTest) TestGetEventByHeight_Error() {
	height := int64(120)

	req := &eventpb.GetByHeightRequest{
		Height: height,
	}
	res := &eventpb.GetByHeightResponse{}

	e := errors.New("new polkadothub-proxy error")

	bc.EventClientMock.On("GetByHeight", mock.AnythingOfType("*context.emptyCtx"), req, mock.AnythingOfType("[]grpc.CallOption")).Return(res, e)

	response, err := bc.GetEventByHeight(uint64(height))

	bc.Require().Nil(response)

	bc.Require().Contains(err.Error(), "Error while getting event by height: 120: new polkadothub-proxy error")
}

func (bc *BlockClientTest) TestGetTransactionByHeight_OK() {
	height := int64(120)

	req := &transactionpb.GetByHeightRequest{
		Height: height,
	}

	res := &transactionpb.GetByHeightResponse{
		Transactions: []*transactionpb.Transaction{
			{
				ExtrinsicIndex: 2,
				Hash:           "0x2326841a64e0a3fff2b4bb760d316cc74b33a8a9480a28ab7e7885acba85e3cf",
			},
		},
	}

	bc.TransactionClientMock.On("GetByHeight", mock.AnythingOfType("*context.emptyCtx"), req, mock.AnythingOfType("[]grpc.CallOption")).Return(res, nil)

	response, err := bc.GetTransactionByHeight(uint64(height))

	bc.Require().Nil(err)

	bc.Require().Len(response.Transactions, 1)
	bc.Require().Equal(res.Transactions[0].ExtrinsicIndex, response.Transactions[0].ExtrinsicIndex)
	bc.Require().Equal(res.Transactions[0].Hash, response.Transactions[0].Hash)
}

func (bc *BlockClientTest) TestGetTransactionByHeight_Error() {
	height := int64(120)

	req := &transactionpb.GetByHeightRequest{
		Height: height,
	}
	res := &transactionpb.GetByHeightResponse{}

	e := errors.New("new polkadothub-proxy error")

	bc.TransactionClientMock.On("GetByHeight", mock.AnythingOfType("*context.emptyCtx"), req, mock.AnythingOfType("[]grpc.CallOption")).Return(res, e)

	response, err := bc.GetTransactionByHeight(uint64(height))

	bc.Require().Nil(response)

	bc.Require().Contains(err.Error(), "Error while getting transaction by height: 120: new polkadothub-proxy error")
}

func TestBlockClient(t *testing.T) {
	suite.Run(t, new(BlockClientTest))
}

type blockClientMock struct {
	mock.Mock
}

func (m blockClientMock) GetByHeight(ctx context.Context, in *blockpb.GetByHeightRequest, opts ...grpc.CallOption) (*blockpb.GetByHeightResponse, error) {
	args := m.Called(ctx, in, opts)
	return args.Get(0).(*blockpb.GetByHeightResponse), args.Error(1)
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

func (m transactionClientMock) GetByHeight(ctx context.Context, in *transactionpb.GetByHeightRequest, opts ...grpc.CallOption) (*transactionpb.GetByHeightResponse, error) {
	args := m.Called(ctx, in, opts)
	return args.Get(0).(*transactionpb.GetByHeightResponse), args.Error(1)
}

func (m transactionClientMock) GetAnnotatedByHeight(ctx context.Context, in *transactionpb.GetAnnotatedByHeightRequest, opts ...grpc.CallOption) (*transactionpb.GetAnnotatedByHeightResponse, error) {
	args := m.Called(ctx, in, opts)
	return args.Get(0).(*transactionpb.GetAnnotatedByHeightResponse), args.Error(1)
}
