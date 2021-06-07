package indexer_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/figment-networks/polkadot-worker/api"
	"github.com/figment-networks/polkadot-worker/api/scale"
	"github.com/figment-networks/polkadot-worker/indexer"
	wStructs "github.com/figment-networks/polkadot-worker/structs"
	"github.com/figment-networks/polkadot-worker/utils"

	"github.com/figment-networks/indexer-manager/structs"
	cStructs "github.com/figment-networks/indexer-manager/worker/connectivity/structs"
	"github.com/figment-networks/polkadothub-proxy/grpc/account/accountpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/block/blockpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/chain/chainpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/decode/decodepb"
	"github.com/figment-networks/polkadothub-proxy/grpc/event/eventpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/transaction/transactionpb"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

type IndexerClientTest struct {
	suite.Suite
	*indexer.Client

	Height   [2]uint64
	ChainID  string
	Currency string
	Exp      int
	ReqID    uuid.UUID
	Version  string

	Block        structs.Block
	Ctx          context.Context
	CtxCancel    context.CancelFunc
	Decoded      decodepb.DecodeResponse
	Transactions transactions

	PolkaMock   PolkaClientMock
	ProxyClient *proxyClientMock
}

func (ic *IndexerClientTest) SetupTest() {
	reqID, err := uuid.NewRandom()
	ic.Require().Nil(err)
	ic.ReqID = reqID

	ic.ChainID = "polkadot"
	ic.Currency = "DOT"
	ic.Exp = 10
	ic.Version = "0.0.1"
	ic.Height = [2]uint64{5400570, 5400570}

	log, err := zap.NewDevelopment()
	ic.Require().Nil(err)

	proxyClientMock := proxyClientMock{}

	ctx, ctxCancel := context.WithCancel(context.Background())
	ic.Ctx = ctx
	ic.CtxCancel = ctxCancel

	ic.PolkaMock = PolkaClientMock{}

	ds := scale.NewDecodeStorage()
	if err := ds.Init("polkadot"); err != nil {
		log.Fatal("Error creating decode storage", zap.Error(err))
	}

	ic.Client = indexer.NewClient(log, &proxyClientMock, ic.Exp, 1000, ic.ChainID, ic.Currency, &ic.PolkaMock, ds)
	ic.ProxyClient = &proxyClientMock
}

func (ic *IndexerClientTest) TestGetAccountBalance_OK() {
	account := "14coxGrE4uD8ZMascmAPXhvggwnp8bgdW2fFVWMZSqJEFxCV"
	height := uint64(23452)
	resp := &accountpb.GetByHeightResponse{
		Account: &accountpb.Account{
			Nonce:      3452,
			FeeFrozen:  "12",
			Free:       "1234",
			Reserved:   "20",
			MiscFrozen: "24",
		},
	}
	ic.ProxyClient.On("GetAccountBalance", mock.AnythingOfType("*context.cancelCtx"), account, height).Return(resp, nil)

	req := structs.HeightAccount{
		Account: account,
		Height:  height,
	}

	var buffer bytes.Buffer
	err := json.NewEncoder(&buffer).Encode(req)
	ic.Require().Nil(err)

	tr := cStructs.TaskRequest{
		Id:      ic.ReqID,
		Type:    structs.ReqIDAccountBalance,
		Payload: make([]byte, buffer.Len()),
	}
	buffer.Read(tr.Payload)

	stream := cStructs.NewStreamAccess()
	defer stream.Close()

	ic.Require().Nil(ic.RegisterStream(ic.Ctx, stream))
	defer ic.Require().Nil(ic.CloseStream(ic.Ctx, stream.StreamID))
	defer ic.CtxCancel()

	ic.Require().Nil(stream.Req(tr))

	accountBalance, endFounded := false, false
	for s := range stream.ResponseListener {
		if ic.ReqID.String() != s.Id.String() {
			continue
		}

		switch s.Type {
		case "AccountBalance":
			var resp structs.GetAccountBalanceResponse
			err := json.Unmarshal(s.Payload, &resp)
			ic.Require().Nil(err)

			ic.Require().Equal(height, resp.Height)
			ic.Require().Len(resp.Balances, 1)

			balance := resp.Balances[0]
			ic.Require().EqualValues(ic.Exp, balance.Exp)
			ic.Require().Equal(ic.Currency, balance.Currency)
			ic.Require().Equal("0.0000001234DOT", balance.Text)
			ic.Require().Equal("1234", balance.Numeric.String())

			accountBalance = true
		case "END":
			ic.Require().True(s.Final)
			endFounded = true
		}

		if accountBalance && endFounded {
			break
		}
	}
}

func (ic *IndexerClientTest) TestGetAccountBalance_ProxyError() {
	account := "14coxGrE4uD8ZMascmAPXhvggwnp8bgdW2fFVWMZSqJEFxCV"
	height := uint64(23452)

	e := errors.New("new balance error")
	ic.ProxyClient.On("GetAccountBalance", mock.AnythingOfType("*context.cancelCtx"), account, height).Return(&accountpb.GetByHeightResponse{}, e)

	req := structs.HeightAccount{
		Account: account,
		Height:  height,
	}

	var buffer bytes.Buffer
	err := json.NewEncoder(&buffer).Encode(req)
	ic.Require().Nil(err)

	tr := cStructs.TaskRequest{
		Id:      ic.ReqID,
		Type:    structs.ReqIDAccountBalance,
		Payload: make([]byte, buffer.Len()),
	}
	buffer.Read(tr.Payload)

	stream := cStructs.NewStreamAccess()
	defer stream.Close()

	ic.Require().Nil(ic.RegisterStream(ic.Ctx, stream))
	defer ic.Require().Nil(ic.CloseStream(ic.Ctx, stream.StreamID))
	defer ic.CtxCancel()

	ic.Require().Nil(stream.Req(tr))

	for response := range stream.ResponseListener {
		if response.Id.String() != ic.ReqID.String() || response.Error.Msg == "" {
			continue
		}

		ic.Require().True(response.Final)
		ic.Require().Contains(response.Error.Msg, "Could not send Account Balance: new balance error")
		return
	}
}

func (ic *IndexerClientTest) TestGetAccountBalance_MapperError() {
	account := "14coxGrE4uD8ZMascmAPXhvggwnp8bgdW2fFVWMZSqJEFxCV"
	height := uint64(23452)
	resp := &accountpb.GetByHeightResponse{
		Account: &accountpb.Account{
			Nonce:      3452,
			FeeFrozen:  "12",
			Free:       "bad",
			Reserved:   "20",
			MiscFrozen: "24",
		},
	}
	ic.ProxyClient.On("GetAccountBalance", mock.AnythingOfType("*context.cancelCtx"), account, height).Return(resp, nil)

	req := structs.HeightAccount{
		Account: account,
		Height:  height,
	}

	var buffer bytes.Buffer
	err := json.NewEncoder(&buffer).Encode(req)
	ic.Require().Nil(err)

	tr := cStructs.TaskRequest{
		Id:      ic.ReqID,
		Type:    structs.ReqIDAccountBalance,
		Payload: make([]byte, buffer.Len()),
	}
	buffer.Read(tr.Payload)

	stream := cStructs.NewStreamAccess()
	defer stream.Close()

	ic.Require().Nil(ic.RegisterStream(ic.Ctx, stream))
	defer ic.Require().Nil(ic.CloseStream(ic.Ctx, stream.StreamID))
	defer ic.CtxCancel()

	ic.Require().Nil(stream.Req(tr))

	for response := range stream.ResponseListener {
		if response.Id.String() != ic.ReqID.String() || response.Error.Msg == "" {
			continue
		}

		ic.Require().True(response.Final)
		ic.Require().Contains(response.Error.Msg, "Could not send Account Balance: Could not create transaction free amount from value \"bad\" Could not create big int from value bad")
		return
	}
}

func (ic *IndexerClientTest) TestGetAccountBalance_UnmarshalError() {
	tr := cStructs.TaskRequest{
		Id:      ic.ReqID,
		Type:    structs.ReqIDAccountBalance,
		Payload: nil,
	}

	stream := cStructs.NewStreamAccess()
	defer stream.Close()

	ic.Require().Nil(ic.RegisterStream(ic.Ctx, stream))
	defer ic.Require().Nil(ic.CloseStream(ic.Ctx, stream.StreamID))
	defer ic.CtxCancel()

	ic.Require().Nil(stream.Req(tr))

	for response := range stream.ResponseListener {
		if response.Id.String() != ic.ReqID.String() || response.Error.Msg == "" {
			continue
		}

		ic.Require().True(response.Final)
		ic.Require().Contains(response.Error.Msg, "Cannot unmarshal payload: bad request")
		return
	}
}

func (ic *IndexerClientTest) TestGetLatest_OK() {
	ic.getLatestRpcResponses()
	ic.ProxyClient.On("DecodeData", mock.AnythingOfType("*context.cancelCtx"), mock.AnythingOfType("structs.DecodeDataRequest"), ic.Height[0]).Return(&ic.Decoded, nil)

	req := structs.LatestDataRequest{
		LastHeight: ic.Height[0],
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

	ic.Require().Nil(ic.RegisterStream(ic.Ctx, stream))
	defer ic.Require().Nil(ic.CloseStream(ic.Ctx, stream.StreamID))
	defer ic.CtxCancel()

	ic.Require().Nil(stream.Req(tr))

	transactions := ic.Transactions.Transactions

	countBlock, countTransaction, endFounded := 0, 0, false
	for s := range stream.ResponseListener {
		if s.Id.String() != ic.ReqID.String() {
			continue
		}
		fmt.Println(s)

		switch s.Type {
		case "Block":
			var block structs.Block
			ic.Require().Nil(json.Unmarshal(s.Payload, &block))
			ic.validateBlock(block, ic.Block)
			countBlock++

		case "Transaction":
			var transaction structs.Transaction
			ic.Require().Nil(json.Unmarshal(s.Payload, &transaction))

			switch transaction.Hash {
			case transactions[0].Hash:
				utils.ValidateTransactions(&ic.Suite, transaction, transactions[0])
			case transactions[1].Hash:
				utils.ValidateTransactions(&ic.Suite, transaction, transactions[1])
			}

			countTransaction++

		case "END":
			ic.Require().True(s.Final)
			endFounded = true
		}

		if countBlock == 1 && countTransaction == 2 && endFounded {
			break
		}
	}
}

func (ic *IndexerClientTest) TestGetLatest_LatestDataRequestUnmarshalError() {
	ic.getLatestRpcResponses()
	tr := cStructs.TaskRequest{
		Id:      ic.ReqID,
		Type:    structs.ReqIDLatestData,
		Payload: make([]byte, 0),
	}

	stream := cStructs.NewStreamAccess()
	defer stream.Close()

	ic.Require().Nil(ic.RegisterStream(ic.Ctx, stream))
	defer ic.Require().Nil(ic.CloseStream(ic.Ctx, stream.StreamID))
	defer ic.CtxCancel()

	ic.Require().Nil(stream.Req(tr))

	for response := range stream.ResponseListener {
		if response.Id.String() != ic.ReqID.String() || response.Error.Msg == "" {
			continue
		}

		ic.Require().True(response.Final)
		ic.Require().Contains(response.Error.Msg, "Cannot unmarshal payload: bad request")
		return
	}
}

func (ic *IndexerClientTest) TestGetLatest_DecodeDataError() {
	ic.getLatestRpcResponses()
	e := errors.New("new decode error")
	ic.ProxyClient.On("DecodeData", mock.AnythingOfType("*context.cancelCtx"), mock.AnythingOfType("structs.DecodeDataRequest"), ic.Height[0]).Return(&decodepb.DecodeResponse{}, e)

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

	ic.Require().Nil(ic.RegisterStream(ic.Ctx, stream))
	defer ic.Require().Nil(ic.CloseStream(ic.Ctx, stream.StreamID))
	defer ic.CtxCancel()

	ic.Require().Nil(stream.Req(tr))

	for response := range stream.ResponseListener {
		fmt.Printf("payload: %v\n", response.Payload)
		if response.Id.String() != ic.ReqID.String() || response.Error.Msg == "" {
			continue
		}

		ic.Require().True(response.Final)
		ic.Require().Contains(response.Error.Msg, "error while decoding data: new decode error")
		return
	}
}

func (ic *IndexerClientTest) TestGetLatest_GetLatestHeightError() {
	ic.getRpcResponses()

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

	ic.Require().Nil(ic.RegisterStream(ic.Ctx, stream))
	defer ic.Require().Nil(ic.CloseStream(ic.Ctx, stream.StreamID))
	defer ic.CtxCancel()

	ic.Require().Nil(stream.Req(tr))

	for response := range stream.ResponseListener {
		if response.Id.String() != ic.ReqID.String() || response.Error.Msg == "" {
			continue
		}

		ic.Require().True(response.Final)
		ic.Require().Contains(response.Error.Msg, "Could not fetch head from proxy: json: cannot unmarshal string into Go value of type scale.BlockHeader")
		return
	}
}

func (ic *IndexerClientTest) TestGetTransactions_OK() {
	var buffer bytes.Buffer
	var block structs.Block
	var transaction structs.Transaction

	ic.getRpcResponses()
	ic.ProxyClient.On("DecodeData", mock.AnythingOfType("*context.cancelCtx"), mock.AnythingOfType("structs.DecodeDataRequest"), ic.Height[0]).Return(&ic.Decoded, nil)

	req := structs.HeightRange{
		StartHeight: uint64(ic.Height[0]),
		EndHeight:   uint64(ic.Height[1]),
	}

	ic.Require().Nil(json.NewEncoder(&buffer).Encode(req))

	tr := cStructs.TaskRequest{
		Id:      ic.ReqID,
		Type:    structs.ReqIDGetTransactions,
		Payload: make([]byte, buffer.Len()),
	}
	buffer.Read(tr.Payload)

	stream := cStructs.NewStreamAccess()
	defer stream.Close()

	ic.Require().Nil(ic.RegisterStream(ic.Ctx, stream))
	defer ic.Require().Nil(ic.CloseStream(ic.Ctx, stream.StreamID))
	defer ic.CtxCancel()

	ic.Require().Nil(stream.Req(tr))

	transactions := ic.Transactions.Transactions

	countBlock, countTransaction, endFounded := 0, 0, false
	for s := range stream.ResponseListener {
		if ic.ReqID != s.Id {
			continue
		}

		switch s.Type {
		case "Block":

			ic.Require().Nil(json.Unmarshal(s.Payload, &block))
			ic.validateBlock(block, ic.Block)

			countBlock++

		case "Transaction":

			ic.Require().Nil(json.Unmarshal(s.Payload, &transaction))

			switch transaction.Hash {
			case transactions[0].Hash:
				utils.ValidateTransactions(&ic.Suite, transaction, transactions[0])
			case transactions[1].Hash:
				utils.ValidateTransactions(&ic.Suite, transaction, transactions[1])
			}

			countTransaction++

		case "END":
			ic.Require().True(s.Final)
			endFounded = true
		}

		if countBlock == 1 && countTransaction == 2 && endFounded {
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

	fmt.Printf("TestGetTransactions_HeightRangeUnmarshalError: %s %s", ic.ReqID, stream.StreamID)

	ic.Require().Nil(ic.RegisterStream(ic.Ctx, stream))
	defer ic.Require().Nil(ic.CloseStream(ic.Ctx, stream.StreamID))
	defer ic.CtxCancel()

	ic.Require().Nil(stream.Req(tr))

	for response := range stream.ResponseListener {
		if response.Id.String() != ic.ReqID.String() || response.Error.Msg == "" {
			continue
		}

		ic.Require().True(response.Final)
		ic.Require().Contains(response.Error.Msg, "Cannot unmarshal payload: bad request")
		return
	}
}

func (ic *IndexerClientTest) TestGetTransactions_DecodeDataError() {
	ic.getRpcResponses()

	e := errors.New("new decode error")
	ic.ProxyClient.On("DecodeData", mock.AnythingOfType("*context.cancelCtx"), mock.AnythingOfType("structs.DecodeDataRequest"), ic.Height[0]).Return(&decodepb.DecodeResponse{}, e)

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

	fmt.Printf("TestGetTransactions_DecodeDataError: %s %s", ic.ReqID, stream.StreamID)

	ic.Require().Nil(ic.RegisterStream(ic.Ctx, stream))
	defer ic.Require().Nil(ic.CloseStream(ic.Ctx, stream.StreamID))
	defer ic.CtxCancel()

	ic.Require().Nil(stream.Req(tr))

	for response := range stream.ResponseListener {
		if response.Id.String() != ic.ReqID.String() || response.Error.Msg == "" {
			continue
		}

		ic.Require().True(response.Final)
		ic.Require().Contains(response.Error.Msg, "error while decoding data: new decode error")
		return
	}
}

func (ic *IndexerClientTest) TestGetTransactions_TransactionMapperError() {
	ic.getRpcResponses()
	ic.Decoded.Block.Block.Extrinsics[0].PartialFee = "bad"

	ic.ProxyClient.On("DecodeData", mock.AnythingOfType("*context.cancelCtx"), mock.AnythingOfType("structs.DecodeDataRequest"), ic.Height[0]).Return(&ic.Decoded, nil)

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

	ic.Require().Nil(ic.RegisterStream(ic.Ctx, stream))
	defer ic.Require().Nil(ic.CloseStream(ic.Ctx, stream.StreamID))
	defer ic.CtxCancel()

	ic.Require().Nil(stream.Req(tr))

	for response := range stream.ResponseListener {
		if response.Id.String() != ic.ReqID.String() || response.Error.Msg == "" {
			continue
		}

		ic.Require().True(response.Final)
		ic.Require().Contains(response.Error.Msg, "error while mapping transactions: Could not parse transaction partial fee \"bad\"")
		return
	}
}

type rpcResponses struct {
	Responses []api.Response
}

type transactions struct {
	Transactions []structs.Transaction
}

type PolkaClientMock struct {
	rpcResp rpcResponses
	lock    sync.Mutex
	rCount  int
}

func (pcm *PolkaClientMock) Send(resp chan api.Response, id uint64, method string, params []interface{}) error {
	pcm.lock.Lock()

	response := pcm.rpcResp.Responses[pcm.rCount]
	resp <- api.Response{
		ID:     response.ID,
		Type:   response.Type,
		Result: response.Result,
	}

	pcm.rCount++
	pcm.lock.Unlock()
	return nil
}

func (ic *IndexerClientTest) getRpcResponses() {
	utils.ReadFile(ic.Suite, "./../utils/rpc_responses.json", &ic.PolkaMock.rpcResp)
	ic.getDecodedData()
}

func (ic *IndexerClientTest) getLatestRpcResponses() {
	utils.ReadFile(ic.Suite, "./../utils/rpc_responses_get_latest.json", &ic.PolkaMock.rpcResp)
	ic.getDecodedData()
}

func (ic *IndexerClientTest) getDecodedData() {
	utils.ReadFile(ic.Suite, "./../utils/block.json", &ic.Block)
	utils.ReadFile(ic.Suite, "./../utils/decoded.json", &ic.Decoded)
	utils.ReadFile(ic.Suite, "./../utils/transactions.json", &ic.Transactions.Transactions)
}

func (ic *IndexerClientTest) validateBlock(block, expected structs.Block) {
	ic.Require().EqualValues(expected.Height, block.Height)
	ic.Require().Equal(expected.Hash, block.Hash)
	ic.Require().Equal(expected.Time.Unix(), block.Time.Unix())
}

func TestIndexerClient(t *testing.T) {
	suite.Run(t, new(IndexerClientTest))
}

type proxyClientMock struct {
	mock.Mock
}

func (m proxyClientMock) GetAccountBalance(ctx context.Context, account string, height uint64) (*accountpb.GetByHeightResponse, error) {
	args := m.Called(ctx, account, height)
	return args.Get(0).(*accountpb.GetByHeightResponse), args.Error(1)
}

func (m proxyClientMock) GetBlockByHeight(ctx context.Context, height uint64) (*blockpb.GetByHeightResponse, error) {
	args := m.Called(ctx, height)
	return args.Get(0).(*blockpb.GetByHeightResponse), args.Error(1)
}

func (m proxyClientMock) GetEventsByHeight(ctx context.Context, height uint64) (*eventpb.GetByHeightResponse, error) {
	args := m.Called(ctx, height)
	return args.Get(0).(*eventpb.GetByHeightResponse), args.Error(1)
}

func (m proxyClientMock) GetMetaByHeight(ctx context.Context, height uint64) (*chainpb.GetMetaByHeightResponse, error) {
	args := m.Called(ctx, height)
	return args.Get(0).(*chainpb.GetMetaByHeightResponse), args.Error(1)
}

func (m proxyClientMock) GetHead(ctx context.Context) (*chainpb.GetHeadResponse, error) {
	args := m.Called(ctx)
	return args.Get(0).(*chainpb.GetHeadResponse), args.Error(1)
}

func (m proxyClientMock) GetTransactionsByHeight(ctx context.Context, height uint64) (*transactionpb.GetByHeightResponse, error) {
	args := m.Called(ctx, height)
	return args.Get(0).(*transactionpb.GetByHeightResponse), args.Error(1)
}

func (m proxyClientMock) DecodeData(ctx context.Context, ddr wStructs.DecodeDataRequest, height uint64) (*decodepb.DecodeResponse, error) {
	args := m.Called(ctx, ddr, height)
	return args.Get(0).(*decodepb.DecodeResponse), args.Error(1)
}
