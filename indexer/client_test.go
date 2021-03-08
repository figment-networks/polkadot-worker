package indexer_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/figment-networks/polkadot-worker/indexer"
	"github.com/figment-networks/polkadot-worker/proxy"
	"github.com/figment-networks/polkadot-worker/utils"

	"github.com/figment-networks/indexer-manager/structs"
	cStructs "github.com/figment-networks/indexer-manager/worker/connectivity/structs"
	"github.com/figment-networks/indexing-engine/metrics"
	"github.com/figment-networks/polkadothub-proxy/grpc/block/blockpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/event/eventpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/transaction/transactionpb"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

type IndexerClientTest struct {
	suite.Suite

	*indexer.Client

	Height               []uint64
	ReqID                uuid.UUID
	BlockResponse        []utils.BlockResp
	EventsResponse       [][]utils.EventsResp
	TransactionsResponse []utils.TransactionsResp

	ChainID     string
	Currency    string
	Exp         int
	Version     string
	ProxyClient *proxyClientMock
}

func (ic *IndexerClientTest) SetupTest() {
	reqID, err := uuid.NewRandom()
	ic.Require().Nil(err)
	ic.ReqID = reqID

	ic.ChainID = "Polkadot"
	ic.Currency = "DOT"
	ic.Exp = 12
	ic.Version = "0.0.1"
	ic.Height = []uint64{3941719, 3941720}

	ic.BlockResponse = utils.GetBlocksResponses(ic.Height)
	ic.EventsResponse = utils.GetEventsResponses(ic.Height)
	ic.TransactionsResponse = utils.GetTransactionsResponses(ic.Height)

	log, err := zap.NewDevelopment()
	ic.Require().Nil(err)

	conversionDuration := metrics.MustNewHistogramWithTags(metrics.HistogramOptions{})
	proxy.BlockConversionDuration = conversionDuration.WithLabels("block")
	proxy.TransactionConversionDuration = conversionDuration.WithLabels("transaction")

	proxyClientMock := proxyClientMock{}

	ic.Client = indexer.NewClient(log.Sugar(), &proxyClientMock, ic.Exp, ic.ChainID, ic.Currency, ic.Version)
	ic.ProxyClient = &proxyClientMock
}

func (ic *IndexerClientTest) TestGetLatest_OK() {
	ic.ProxyClient.On("GetBlockByHeight", mock.AnythingOfType("*context.cancelCtx"), ic.Height[0]).Return(utils.BlockResponse(ic.BlockResponse[0]), nil)
	ic.ProxyClient.On("GetTransactionsByHeight", mock.AnythingOfType("*context.cancelCtx"), ic.Height[0]).Return(utils.TransactionsResponse(ic.TransactionsResponse[0]), nil)
	ic.ProxyClient.On("GetEventsByHeight", mock.AnythingOfType("*context.cancelCtx"), ic.Height[0]).Return(utils.EventsResponse(ic.EventsResponse[0]), nil)

	req := structs.LatestDataRequest{
		LastHeight: uint64(ic.Height[0]),
	}

	var buffer bytes.Buffer
	err := json.NewEncoder(&buffer).Encode(req)
	ic.Require().Nil(err)

	tr := cStructs.TaskRequest{
		Id:      ic.ReqID,
		Type:    structs.ReqIDLatestData,
		Payload: make([]byte, buffer.Len()),
	}
	buffer.Read(tr.Payload)

	stream := cStructs.NewStreamAccess()
	defer stream.Close()

	ic.Require().Nil(ic.RegisterStream(context.Background(), stream))
	defer ic.Require().Nil(ic.CloseStream(context.Background(), stream.StreamID))

	stream.RequestListener <- tr

	blockFounded, transactionFounded, endFounded := false, false, false
	for s := range stream.ResponseListener {
		if ic.ReqID != s.Id {
			continue
		}

		switch s.Type {
		case "Block":
			var block structs.Block
			err := json.Unmarshal(s.Payload, &block)
			ic.Require().Nil(err)

			ic.validateBlock(block, ic.BlockResponse[0])
			blockFounded = true
			break

		case "Transaction":
			var transaction structs.Transaction
			err := json.Unmarshal(s.Payload, &transaction)
			ic.Require().Nil(err)

			expectedEvents := ic.EventsResponse[0][1:]
			utils.ValidateTransactions(&ic.Suite, transaction, ic.BlockResponse[0], ic.TransactionsResponse[0], expectedEvents, ic.ChainID, ic.Currency, int32(ic.Exp))
			transactionFounded = true
			break

		case "END":
			ic.Require().True(s.Final)
			endFounded = true
		}

		if blockFounded && transactionFounded && endFounded {
			break
		}
	}
}

func (ic *IndexerClientTest) TestGetLatest_LatestDataRequestUnmarshalError() {
	tr := cStructs.TaskRequest{
		Id:      ic.ReqID,
		Type:    structs.ReqIDLatestData,
		Payload: make([]byte, 0),
	}

	stream := cStructs.NewStreamAccess()
	defer stream.Close()

	ic.Require().Nil(ic.RegisterStream(context.Background(), stream))
	defer ic.CloseStream(context.Background(), stream.StreamID)

	stream.RequestListener <- tr

	for response := range stream.ResponseListener {
		if response.Id != ic.ReqID || response.Error.Msg == "" {
			continue
		}

		ic.Require().True(response.Final)
		ic.Require().Contains(response.Error.Msg, "Cannot unmarshal payload: bad request")
		return
	}
}

func (ic *IndexerClientTest) TestGetLatest_BlockResponseError() {
	e := errors.New("new block error")
	ic.ProxyClient.On("GetBlockByHeight", mock.AnythingOfType("*context.cancelCtx"), ic.Height[0]).Return(&blockpb.GetByHeightResponse{}, e)

	req := structs.LatestDataRequest{
		LastHeight: uint64(ic.Height[0]),
	}

	var buffer bytes.Buffer
	err := json.NewEncoder(&buffer).Encode(req)
	ic.Require().Nil(err)

	tr := cStructs.TaskRequest{
		Id:      ic.ReqID,
		Type:    structs.ReqIDLatestData,
		Payload: make([]byte, buffer.Len()),
	}
	buffer.Read(tr.Payload)

	stream := cStructs.NewStreamAccess()
	defer stream.Close()

	ic.Require().Nil(ic.RegisterStream(context.Background(), stream))
	defer ic.CloseStream(context.Background(), stream.StreamID)

	stream.RequestListener <- tr

	for response := range stream.ResponseListener {
		if response.Id != ic.ReqID || response.Error.Msg == "" {
			continue
		}

		ic.Require().True(response.Final)
		ic.Require().Contains(response.Error.Msg, "Could not fetch latest transactions: Error while getting transactions: new block error")
		return
	}
}

func (ic *IndexerClientTest) TestGetLatest_TransactionResponseError() {
	ic.ProxyClient.On("GetBlockByHeight", mock.AnythingOfType("*context.cancelCtx"), ic.Height[0]).Return(utils.BlockResponse(ic.BlockResponse[0]), nil)

	e := errors.New("new transaction error")
	ic.ProxyClient.On("GetTransactionsByHeight", mock.AnythingOfType("*context.cancelCtx"), ic.Height[0]).Return(&transactionpb.GetByHeightResponse{}, e)

	req := structs.LatestDataRequest{
		LastHeight: uint64(ic.Height[0]),
	}

	var buffer bytes.Buffer
	err := json.NewEncoder(&buffer).Encode(req)
	ic.Require().Nil(err)

	tr := cStructs.TaskRequest{
		Id:      ic.ReqID,
		Type:    structs.ReqIDLatestData,
		Payload: make([]byte, buffer.Len()),
	}
	buffer.Read(tr.Payload)

	stream := cStructs.NewStreamAccess()
	defer stream.Close()

	ic.Require().Nil(ic.RegisterStream(context.Background(), stream))
	defer ic.CloseStream(context.Background(), stream.StreamID)

	stream.RequestListener <- tr

	for response := range stream.ResponseListener {
		if response.Id != ic.ReqID || response.Error.Msg == "" {
			continue
		}

		ic.Require().True(response.Final)
		ic.Require().Contains(response.Error.Msg, "Could not fetch latest transactions: Error while getting transactions: new transaction error")
		return
	}
}

func (ic *IndexerClientTest) TestGetLatest_EventResponseError() {
	ic.ProxyClient.On("GetBlockByHeight", mock.AnythingOfType("*context.cancelCtx"), ic.Height[0]).Return(utils.BlockResponse(ic.BlockResponse[0]), nil)
	ic.ProxyClient.On("GetTransactionsByHeight", mock.AnythingOfType("*context.cancelCtx"), ic.Height[0]).Return(utils.TransactionsResponse(ic.TransactionsResponse[0]), nil)

	e := errors.New("new event error")
	ic.ProxyClient.On("GetEventsByHeight", mock.AnythingOfType("*context.cancelCtx"), ic.Height[0]).Return(&eventpb.GetByHeightResponse{}, e)

	req := structs.LatestDataRequest{
		LastHeight: uint64(ic.Height[0]),
	}

	var buffer bytes.Buffer
	err := json.NewEncoder(&buffer).Encode(req)
	require.Nil(ic.T(), err)

	tr := cStructs.TaskRequest{
		Id:      ic.ReqID,
		Type:    structs.ReqIDLatestData,
		Payload: make([]byte, buffer.Len()),
	}
	buffer.Read(tr.Payload)

	stream := cStructs.NewStreamAccess()
	defer stream.Close()

	ic.Require().Nil(ic.RegisterStream(context.Background(), stream))
	defer ic.CloseStream(context.Background(), stream.StreamID)

	stream.RequestListener <- tr

	for response := range stream.ResponseListener {
		if response.Id != ic.ReqID || response.Error.Msg == "" {
			continue
		}

		ic.Require().True(response.Final)
		ic.Require().Contains(response.Error.Msg, "Could not fetch latest transactions: Error while getting transactions: new event error")
		return
	}
}

func (ic *IndexerClientTest) TestGetTransactions_OK() {
	ic.ProxyClient.On("GetBlockByHeight", mock.AnythingOfType("*context.cancelCtx"), ic.Height[0]).Return(utils.BlockResponse(ic.BlockResponse[0]), nil)
	ic.ProxyClient.On("GetBlockByHeight", mock.AnythingOfType("*context.cancelCtx"), ic.Height[1]).Return(utils.BlockResponse(ic.BlockResponse[1]), nil)
	ic.ProxyClient.On("GetTransactionsByHeight", mock.AnythingOfType("*context.cancelCtx"), ic.Height[0]).Return(utils.TransactionsResponse(ic.TransactionsResponse[0]), nil)
	ic.ProxyClient.On("GetTransactionsByHeight", mock.AnythingOfType("*context.cancelCtx"), ic.Height[1]).Return(utils.TransactionsResponse(ic.TransactionsResponse[1]), nil)
	ic.ProxyClient.On("GetEventsByHeight", mock.AnythingOfType("*context.cancelCtx"), ic.Height[0]).Return(utils.EventsResponse(ic.EventsResponse[0]), nil)
	ic.ProxyClient.On("GetEventsByHeight", mock.AnythingOfType("*context.cancelCtx"), ic.Height[1]).Return(utils.EventsResponse(ic.EventsResponse[1]), nil)

	req := structs.HeightRange{
		StartHeight: uint64(ic.Height[0]),
		EndHeight:   uint64(ic.Height[1]),
	}

	var buffer bytes.Buffer
	err := json.NewEncoder(&buffer).Encode(req)
	ic.Require().Nil(err)

	tr := cStructs.TaskRequest{
		Id:      ic.ReqID,
		Type:    structs.ReqIDGetTransactions,
		Payload: make([]byte, buffer.Len()),
	}
	buffer.Read(tr.Payload)

	stream := cStructs.NewStreamAccess()
	defer stream.Close()

	ic.Require().Nil(ic.RegisterStream(context.Background(), stream))
	defer ic.CloseStream(context.Background(), stream.StreamID)

	stream.RequestListener <- tr

	countBlock, countTransaction, endFounded := 0, 0, false
	for s := range stream.ResponseListener {
		if ic.ReqID != s.Id {
			continue
		}

		switch s.Type {
		case "Block":
			var block structs.Block
			err := json.Unmarshal(s.Payload, &block)
			ic.Require().Nil(err)

			switch block.Hash {
			case ic.BlockResponse[0].Hash:
				ic.validateBlock(block, ic.BlockResponse[0])
				break
			case ic.BlockResponse[1].Hash:
				ic.validateBlock(block, ic.BlockResponse[1])
			}

			countBlock++
			break

		case "Transaction":
			var transaction structs.Transaction
			err := json.Unmarshal(s.Payload, &transaction)
			ic.Require().Nil(err)

			switch transaction.Hash {
			case ic.TransactionsResponse[0].Hash:
				expectedEvents := ic.EventsResponse[0][1:]
				utils.ValidateTransactions(&ic.Suite, transaction, ic.BlockResponse[0], ic.TransactionsResponse[0], expectedEvents, ic.ChainID, ic.Currency, int32(ic.Exp))
				break
			case ic.TransactionsResponse[1].Hash:
				expectedEvents := ic.EventsResponse[1][1:]
				utils.ValidateTransactions(&ic.Suite, transaction, ic.BlockResponse[1], ic.TransactionsResponse[1], expectedEvents, ic.ChainID, ic.Currency, int32(ic.Exp))
			}
			countTransaction++
			break

		case "END":
			endFounded = true
			ic.Require().True(s.Final)
		}

		if endFounded && countBlock == 2 && countTransaction == 2 {
			break
		}
	}
}

func (ic *IndexerClientTest) TestGetTransactions_HeightRangeUnmarshalError() {
	tr := cStructs.TaskRequest{
		Id:      ic.ReqID,
		Type:    structs.ReqIDGetTransactions,
		Payload: make([]byte, 0),
	}

	stream := cStructs.NewStreamAccess()
	defer stream.Close()

	ic.Require().Nil(ic.RegisterStream(context.Background(), stream))
	defer ic.CloseStream(context.Background(), stream.StreamID)

	stream.RequestListener <- tr

	for response := range stream.ResponseListener {
		if response.Id != ic.ReqID || response.Error.Msg == "" {
			continue
		}

		ic.Require().True(response.Final)
		ic.Require().Contains(response.Error.Msg, "Cannot unmarshal payload: bad request")
		return
	}
}

func (ic *IndexerClientTest) TestGetTransactions_GetBlockByHeightError() {
	ic.ProxyClient.On("GetBlockByHeight", mock.AnythingOfType("*context.cancelCtx"), ic.Height[0]).Return(utils.BlockResponse(ic.BlockResponse[0]), nil)
	ic.ProxyClient.On("GetTransactionsByHeight", mock.AnythingOfType("*context.cancelCtx"), ic.Height[0]).Return(utils.TransactionsResponse(ic.TransactionsResponse[0]), nil)
	ic.ProxyClient.On("GetTransactionsByHeight", mock.AnythingOfType("*context.cancelCtx"), ic.Height[1]).Return(utils.TransactionsResponse(ic.TransactionsResponse[1]), nil)
	ic.ProxyClient.On("GetEventsByHeight", mock.AnythingOfType("*context.cancelCtx"), ic.Height[0]).Return(utils.EventsResponse(ic.EventsResponse[0]), nil)
	ic.ProxyClient.On("GetEventsByHeight", mock.AnythingOfType("*context.cancelCtx"), ic.Height[1]).Return(utils.EventsResponse(ic.EventsResponse[1]), nil)

	e := errors.New("new block error")
	ic.ProxyClient.On("GetBlockByHeight", mock.AnythingOfType("*context.cancelCtx"), ic.Height[1]).Return(&blockpb.GetByHeightResponse{}, e)

	req := structs.HeightRange{
		StartHeight: uint64(ic.Height[0]),
		EndHeight:   uint64(ic.Height[1]),
	}

	var buffer bytes.Buffer
	err := json.NewEncoder(&buffer).Encode(req)
	ic.Require().Nil(err)

	tr := cStructs.TaskRequest{
		Id:      ic.ReqID,
		Type:    structs.ReqIDGetTransactions,
		Payload: make([]byte, buffer.Len()),
	}
	buffer.Read(tr.Payload)

	stream := cStructs.NewStreamAccess()
	defer stream.Close()

	ic.Require().Nil(ic.RegisterStream(context.Background(), stream))
	defer ic.CloseStream(context.Background(), stream.StreamID)

	stream.RequestListener <- tr

	for response := range stream.ResponseListener {
		if response.Id != ic.ReqID || response.Error.Msg == "" {
			continue
		}

		ic.Require().True(response.Final)
		ic.Require().Contains(response.Error.Msg, "Error while getting Transactions with given range: Error while getting transactions: new block error")
		return
	}
}

func (ic *IndexerClientTest) TestGetTransactions_GetTransactionByHeightError() {
	ic.ProxyClient.On("GetBlockByHeight", mock.AnythingOfType("*context.cancelCtx"), ic.Height[0]).Return(utils.BlockResponse(ic.BlockResponse[0]), nil)
	ic.ProxyClient.On("GetBlockByHeight", mock.AnythingOfType("*context.cancelCtx"), ic.Height[1]).Return(utils.BlockResponse(ic.BlockResponse[1]), nil)
	ic.ProxyClient.On("GetEventsByHeight", mock.AnythingOfType("*context.cancelCtx"), ic.Height[0]).Return(utils.EventsResponse(ic.EventsResponse[0]), nil)
	ic.ProxyClient.On("GetEventsByHeight", mock.AnythingOfType("*context.cancelCtx"), ic.Height[1]).Return(utils.EventsResponse(ic.EventsResponse[1]), nil)

	e := errors.New("new transaction error")
	ic.ProxyClient.On("GetTransactionsByHeight", mock.AnythingOfType("*context.cancelCtx"), ic.Height[0]).Return(&transactionpb.GetByHeightResponse{}, e)

	req := structs.HeightRange{
		StartHeight: uint64(ic.Height[0]),
		EndHeight:   uint64(ic.Height[0]),
	}

	var buffer bytes.Buffer
	err := json.NewEncoder(&buffer).Encode(req)
	ic.Require().Nil(err)

	tr := cStructs.TaskRequest{
		Id:      ic.ReqID,
		Type:    structs.ReqIDGetTransactions,
		Payload: make([]byte, buffer.Len()),
	}
	buffer.Read(tr.Payload)

	stream := cStructs.NewStreamAccess()
	defer stream.Close()

	ic.Require().Nil(ic.RegisterStream(context.Background(), stream))
	defer ic.CloseStream(context.Background(), stream.StreamID)

	stream.RequestListener <- tr

	for response := range stream.ResponseListener {
		if response.Id != ic.ReqID || response.Error.Msg == "" {
			continue
		}

		ic.Require().True(response.Final)

		errMsg := response.Error.Msg
		ic.Require().Contains(errMsg, "Error while getting Transactions with given range: Error while getting transactions")
		ic.Require().Contains(errMsg, "new transaction error")
		return
	}
}

func (ic *IndexerClientTest) TestGetTransactions_GetEventByHeightError() {
	ic.ProxyClient.On("GetBlockByHeight", mock.AnythingOfType("*context.cancelCtx"), ic.Height[0]).Return(utils.BlockResponse(ic.BlockResponse[0]), nil)
	ic.ProxyClient.On("GetBlockByHeight", mock.AnythingOfType("*context.cancelCtx"), ic.Height[1]).Return(utils.BlockResponse(ic.BlockResponse[1]), nil)
	ic.ProxyClient.On("GetTransactionsByHeight", mock.AnythingOfType("*context.cancelCtx"), ic.Height[0]).Return(utils.TransactionsResponse(ic.TransactionsResponse[0]), nil)
	ic.ProxyClient.On("GetTransactionsByHeight", mock.AnythingOfType("*context.cancelCtx"), ic.Height[1]).Return(utils.TransactionsResponse(ic.TransactionsResponse[1]), nil)

	e := errors.New("new event error one")
	ic.ProxyClient.On("GetEventsByHeight", mock.AnythingOfType("*context.cancelCtx"), ic.Height[0]).Return(&eventpb.GetByHeightResponse{}, e)

	e2 := errors.New("new event error two")
	ic.ProxyClient.On("GetEventsByHeight", mock.AnythingOfType("*context.cancelCtx"), ic.Height[1]).Return(&eventpb.GetByHeightResponse{}, e2)

	req := structs.HeightRange{
		StartHeight: uint64(ic.Height[0]),
		EndHeight:   uint64(ic.Height[1]),
	}

	var buffer bytes.Buffer
	err := json.NewEncoder(&buffer).Encode(req)
	ic.Require().Nil(err)

	tr := cStructs.TaskRequest{
		Id:      ic.ReqID,
		Type:    structs.ReqIDGetTransactions,
		Payload: make([]byte, buffer.Len()),
	}
	buffer.Read(tr.Payload)

	stream := cStructs.NewStreamAccess()
	defer stream.Close()

	ic.Require().Nil(ic.RegisterStream(context.Background(), stream))
	defer ic.CloseStream(context.Background(), stream.StreamID)

	stream.RequestListener <- tr

	for response := range stream.ResponseListener {
		if response.Id != ic.ReqID || response.Error.Msg == "" {
			continue
		}

		errMsg := response.Error.Msg
		ic.Require().Contains(errMsg, "Error while getting Transactions with given range: Error while getting transactions")
		ic.Require().Contains(errMsg, "new event error one")
		ic.Require().Contains(errMsg, "new event error two")
		return
	}
}

func (ic *IndexerClientTest) TestGetTransactions_TransactionMapperError() {
	ic.TransactionsResponse[0].Fee = "bad"

	ic.ProxyClient.On("GetBlockByHeight", mock.AnythingOfType("*context.cancelCtx"), ic.Height[0]).Return(utils.BlockResponse(ic.BlockResponse[0]), nil)
	ic.ProxyClient.On("GetTransactionsByHeight", mock.AnythingOfType("*context.cancelCtx"), ic.Height[0]).Return(utils.TransactionsResponse(ic.TransactionsResponse[0]), nil)
	ic.ProxyClient.On("GetEventsByHeight", mock.AnythingOfType("*context.cancelCtx"), ic.Height[0]).Return(utils.EventsResponse(ic.EventsResponse[0]), nil)

	req := structs.HeightRange{
		StartHeight: uint64(ic.Height[0]),
		EndHeight:   uint64(ic.Height[0]),
	}

	var buffer bytes.Buffer
	err := json.NewEncoder(&buffer).Encode(req)
	ic.Require().Nil(err)

	tr := cStructs.TaskRequest{
		Id:      ic.ReqID,
		Type:    structs.ReqIDGetTransactions,
		Payload: make([]byte, buffer.Len()),
	}
	buffer.Read(tr.Payload)

	stream := cStructs.NewStreamAccess()
	defer stream.Close()

	ic.Require().Nil(ic.RegisterStream(context.Background(), stream))
	defer ic.CloseStream(context.Background(), stream.StreamID)

	stream.RequestListener <- tr

	for response := range stream.ResponseListener {
		if response.Id != ic.ReqID || response.Error.Msg == "" {
			continue
		}

		ic.Require().True(response.Final)
		ic.Require().Contains(response.Error.Msg, "Error while getting Transactions with given range: Error while getting transactions: Could not parse transaction partial fee \"bad\"")
		return
	}
}

func (ic *IndexerClientTest) validateBlock(block structs.Block, resp utils.BlockResp) {
	ic.Require().EqualValues(resp.Height, block.Height)
	ic.Require().Equal(resp.Hash, block.Hash)
	ic.Require().Equal(resp.Time.Seconds, block.Time.Unix())
}

func TestIndexerClient(t *testing.T) {
	suite.Run(t, new(IndexerClientTest))
}

type proxyClientMock struct {
	mock.Mock
}

func (m proxyClientMock) GetBlockByHeight(ctx context.Context, height uint64) (*blockpb.GetByHeightResponse, error) {
	args := m.Called(ctx, height)
	return args.Get(0).(*blockpb.GetByHeightResponse), args.Error(1)
}

func (m proxyClientMock) GetEventsByHeight(ctx context.Context, height uint64) (*eventpb.GetByHeightResponse, error) {
	args := m.Called(ctx, height)
	return args.Get(0).(*eventpb.GetByHeightResponse), args.Error(1)
}

func (m proxyClientMock) GetTransactionsByHeight(ctx context.Context, height uint64) (*transactionpb.GetByHeightResponse, error) {
	args := m.Called(ctx, height)
	return args.Get(0).(*transactionpb.GetByHeightResponse), args.Error(1)
}
