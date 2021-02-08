package indexer_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/figment-networks/indexing-engine/metrics"
	"github.com/figment-networks/polkadot-worker/worker/indexer"
	"github.com/figment-networks/polkadot-worker/worker/proxy"
	"github.com/figment-networks/polkadot-worker/worker/utils"

	"github.com/figment-networks/indexer-manager/structs"
	cStructs "github.com/figment-networks/indexer-manager/worker/connectivity/structs"
	"github.com/figment-networks/polkadothub-proxy/grpc/block/blockpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/event/eventpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/transaction/transactionpb"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var chainID = "chainID"

type IndexerClientTest struct {
	suite.Suite

	*indexer.Client

	ProxyClient *proxyClientMock
}

func (ic *IndexerClientTest) SetupTest() {
	page := uint64(100)

	log, err := zap.NewDevelopment()
	ic.Require().Nil(err)

	conversionDuration := metrics.MustNewHistogramWithTags(metrics.HistogramOptions{})
	proxy.BlockConversionDuration = conversionDuration.WithLabels("block")
	proxy.TransactionConversionDuration = conversionDuration.WithLabels("transaction")

	proxyClientMock := proxyClientMock{}

	ic.Client = indexer.NewClient(chainID, log.Sugar(), page, &proxyClientMock)
	ic.ProxyClient = &proxyClientMock
}

func (ic *IndexerClientTest) TestGetLatest_OK() {
	blockHash := "0x2326841a64e0a3fff2b4bb760d316cc74b33a8a9480a28ab7e7885acba85e3cf"
	trHash := "0xe05668b03c6e8bc1dbb61afab01d0515460693e5ade5627461c6298fcf6b8e72"
	trExtrinsicIndex := int64(7)
	evIds := [][2]int64{{1, 3}, {7, 4}}
	fee := "1000"
	height := uint64(7926)
	isSuccess := true
	now := time.Now()
	time := strconv.Itoa(int(now.Unix()))
	reqID, err := uuid.NewRandom()
	ic.Require().Nil(err)

	blockRes := utils.BlockResponse(int64(height), blockHash, timestamppb.New(now))
	ic.ProxyClient.On("GetBlockByHeight", height).Return(blockRes, nil)

	trRes := utils.TransactionResponse(trExtrinsicIndex, isSuccess, fee, trHash, time)
	ic.ProxyClient.On("GetTransactionByHeight", height).Return(trRes, nil)

	evRes := utils.EventResponse(evIds)
	ic.ProxyClient.On("GetEventByHeight", height).Return(evRes, nil)

	req := structs.LatestDataRequest{
		LastHeight: height,
	}

	var buffer bytes.Buffer
	err = json.NewEncoder(&buffer).Encode(req)
	ic.Require().Nil(err)

	tr := cStructs.TaskRequest{
		Id:      reqID,
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
		if reqID != s.Id {
			continue
		}

		switch s.Type {
		case "Block":
			var block structs.Block
			err := json.Unmarshal(s.Payload, &block)
			ic.Require().Nil(err)

			ic.validateBlock(block, height, blockHash, now)
			blockFounded = true
			break

		case "Transaction":
			var transaction structs.Transaction
			err := json.Unmarshal(s.Payload, &transaction)
			ic.Require().Nil(err)

			ic.validateTransaction(transaction, height, blockHash, trHash, fee, isSuccess, now, []string{"4"})
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
	reqID, err := uuid.NewRandom()
	ic.Require().Nil(err)

	tr := cStructs.TaskRequest{
		Id:      reqID,
		Type:    structs.ReqIDLatestData,
		Payload: make([]byte, 0),
	}

	stream := cStructs.NewStreamAccess()
	defer stream.Close()

	ic.Require().Nil(ic.RegisterStream(context.Background(), stream))
	defer ic.CloseStream(context.Background(), stream.StreamID)

	stream.RequestListener <- tr

	for response := range stream.ResponseListener {
		if response.Id != reqID || response.Error.Msg == "" {
			continue
		}

		ic.Require().True(response.Final)
		ic.Require().Contains(response.Error.Msg, "Cannot unmarshal payload: bad request")
		return
	}
}

func (ic *IndexerClientTest) TestGetLatest_BlockResponseError() {
	height := uint64(7926)
	reqID, err := uuid.NewRandom()
	ic.Require().Nil(err)

	blockRes := &blockpb.GetByHeightResponse{}
	e := errors.New("new block error")
	ic.ProxyClient.On("GetBlockByHeight", height).Return(blockRes, e)

	req := structs.LatestDataRequest{
		LastHeight: height,
	}

	var buffer bytes.Buffer
	err = json.NewEncoder(&buffer).Encode(req)
	ic.Require().Nil(err)

	tr := cStructs.TaskRequest{
		Id:      reqID,
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
		if response.Id != reqID || response.Error.Msg == "" {
			continue
		}

		ic.Require().True(response.Final)
		ic.Require().Contains(response.Error.Msg, "Could not fetch latest transactions: Error while getting transactions: new block error")
		return
	}
}

func (ic *IndexerClientTest) TestGetLatest_TransactionResponseError() {
	blockHash := "0x2326841a64e0a3fff2b4bb760d316cc74b33a8a9480a28ab7e7885acba85e3cf"
	height := uint64(7926)
	now := time.Now()
	reqID, err := uuid.NewRandom()
	ic.Require().Nil(err)

	blockRes := utils.BlockResponse(int64(height), blockHash, timestamppb.New(now))
	ic.ProxyClient.On("GetBlockByHeight", height).Return(blockRes, nil)

	trRes := &transactionpb.GetByHeightResponse{}
	e := errors.New("new transaction error")
	ic.ProxyClient.On("GetTransactionByHeight", height).Return(trRes, e)

	req := structs.LatestDataRequest{
		LastHeight: height,
	}

	var buffer bytes.Buffer
	err = json.NewEncoder(&buffer).Encode(req)
	ic.Require().Nil(err)

	tr := cStructs.TaskRequest{
		Id:      reqID,
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
		if response.Id != reqID || response.Error.Msg == "" {
			continue
		}

		ic.Require().True(response.Final)
		ic.Require().Contains(response.Error.Msg, "Could not fetch latest transactions: Error while getting transactions: new transaction error")
		return
	}
}

func (ic *IndexerClientTest) TestGetLatest_EventResponseError() {
	blockHash := "0x2326841a64e0a3fff2b4bb760d316cc74b33a8a9480a28ab7e7885acba85e3cf"
	trHash := "0xe05668b03c6e8bc1dbb61afab01d0515460693e5ade5627461c6298fcf6b8e72"
	trExtrinsicIndex := int64(7)
	fee := "1000"
	height := uint64(7926)
	isSuccess := true
	now := time.Now()
	time := strconv.Itoa(int(now.Unix()))
	reqID, err := uuid.NewRandom()
	ic.Require().Nil(err)

	blockRes := utils.BlockResponse(int64(height), blockHash, timestamppb.New(now))
	ic.ProxyClient.On("GetBlockByHeight", height).Return(blockRes, nil)

	trRes := utils.TransactionResponse(trExtrinsicIndex, isSuccess, fee, trHash, time)
	ic.ProxyClient.On("GetTransactionByHeight", height).Return(trRes, nil)

	evRes := &eventpb.GetByHeightResponse{}
	e := errors.New("new event error")
	ic.ProxyClient.On("GetEventByHeight", height).Return(evRes, e)

	req := structs.LatestDataRequest{
		LastHeight: height,
	}

	var buffer bytes.Buffer
	err = json.NewEncoder(&buffer).Encode(req)
	require.Nil(ic.T(), err)

	tr := cStructs.TaskRequest{
		Id:      reqID,
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
		if response.Id != reqID || response.Error.Msg == "" {
			continue
		}

		ic.Require().True(response.Final)
		ic.Require().Contains(response.Error.Msg, "Could not fetch latest transactions: Error while getting transactions: new event error")
		return
	}
}

func (ic *IndexerClientTest) TestGetTransactions_OK() {
	startHeight := uint64(7926)
	endHeight := uint64(7927)
	now := time.Now()
	reqID, err := uuid.NewRandom()
	ic.Require().Nil(err)

	blockHash1 := "0x2326841a64e0a3fff2b4bb760d316cc74b33a8a9480a28ab7e7885acba85e3cf"
	trHash1 := "0xe05668b03c6e8bc1dbb61afab01d0515460693e5ade5627461c6298fcf6b8e72"
	trExtrinsicIndex1 := int64(7)
	evIds1 := [][2]int64{{1, 3}, {7, 4}}
	fee1 := "1000"
	isSuccess1 := true
	time1 := strconv.Itoa(int(now.Unix()))

	time2 := now.Add(-500 * time.Second)
	blockHash2 := "316cc74b33a8a9480a28ab7e7885acba0x2326841a64e0a3fff2b4bb760d85e3cf"
	trHash2 := "5668b03c61afab01d05150xe0460693e56e8bc1dbbade5627461c6298fcf6b8e72"
	trExtrinsicIndex2 := int64(21)
	evIds2 := [][2]int64{{21, 63}, {11, 77}}
	fee2 := "2000"
	isSuccess2 := false
	time2Str := strconv.Itoa(int(time2.Unix()))

	blockRes := utils.BlockResponse(int64(startHeight), blockHash1, timestamppb.New(now))
	ic.ProxyClient.On("GetBlockByHeight", startHeight).Return(blockRes, nil)

	blockRes2 := utils.BlockResponse(int64(endHeight), blockHash2, timestamppb.New(time2))
	ic.ProxyClient.On("GetBlockByHeight", endHeight).Return(blockRes2, nil)

	trRes := utils.TransactionResponse(trExtrinsicIndex1, isSuccess1, fee1, trHash1, time1)
	ic.ProxyClient.On("GetTransactionByHeight", startHeight).Return(trRes, nil)

	trRes2 := utils.TransactionResponse(trExtrinsicIndex2, isSuccess2, fee2, trHash2, time2Str)
	ic.ProxyClient.On("GetTransactionByHeight", endHeight).Return(trRes2, nil)

	evRes := utils.EventResponse(evIds1)
	ic.ProxyClient.On("GetEventByHeight", startHeight).Return(evRes, nil)

	evRes2 := utils.EventResponse(evIds2)
	ic.ProxyClient.On("GetEventByHeight", endHeight).Return(evRes2, nil)

	req := structs.HeightRange{
		StartHeight: startHeight,
		EndHeight:   endHeight,
	}

	var buffer bytes.Buffer
	err = json.NewEncoder(&buffer).Encode(req)
	ic.Require().Nil(err)

	tr := cStructs.TaskRequest{
		Id:      reqID,
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
		if reqID != s.Id {
			continue
		}

		switch s.Type {
		case "Block":
			var block structs.Block
			err := json.Unmarshal(s.Payload, &block)
			ic.Require().Nil(err)

			switch block.Hash {
			case blockHash1:
				ic.validateBlock(block, startHeight, blockHash1, now)
				break
			case blockHash2:
				ic.validateBlock(block, endHeight, blockHash2, time2)
			}

			countBlock++
			break

		case "Transaction":
			var transaction structs.Transaction
			err := json.Unmarshal(s.Payload, &transaction)
			ic.Require().Nil(err)

			switch transaction.Hash {
			case trHash1:
				ic.validateTransaction(transaction, startHeight, blockHash1, trHash1, fee1, isSuccess1, now, []string{"4"})
				break
			case trHash2:
				ic.validateTransaction(transaction, endHeight, blockHash2, trHash2, fee2, isSuccess2, time2, []string{"63"})
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
	reqID, err := uuid.NewRandom()
	ic.Require().Nil(err)

	tr := cStructs.TaskRequest{
		Id:      reqID,
		Type:    structs.ReqIDGetTransactions,
		Payload: make([]byte, 0),
	}

	stream := cStructs.NewStreamAccess()
	defer stream.Close()

	ic.Require().Nil(ic.RegisterStream(context.Background(), stream))
	defer ic.CloseStream(context.Background(), stream.StreamID)

	stream.RequestListener <- tr

	for response := range stream.ResponseListener {
		if response.Id != reqID || response.Error.Msg == "" {
			continue
		}

		ic.Require().True(response.Final)
		ic.Require().Contains(response.Error.Msg, "Cannot unmarshal payload: bad request")
		return
	}
}

func (ic *IndexerClientTest) TestGetTransactions_GetBlockByHeightError() {
	startHeight := uint64(7926)
	endHeight := uint64(7927)
	now := time.Now()
	reqID, err := uuid.NewRandom()
	ic.Require().Nil(err)

	blockHash1 := "0x2326841a64e0a3fff2b4bb760d316cc74b33a8a9480a28ab7e7885acba85e3cf"
	trHash1 := "0xe05668b03c6e8bc1dbb61afab01d0515460693e5ade5627461c6298fcf6b8e72"
	trExtrinsicIndex1 := int64(7)
	evIds1 := [][2]int64{{1, 3}, {7, 4}}
	fee1 := "1000"
	isSuccess1 := true
	time1 := strconv.Itoa(int(now.Unix()))

	blockRes := utils.BlockResponse(int64(startHeight), blockHash1, timestamppb.New(now))
	ic.ProxyClient.On("GetBlockByHeight", startHeight).Return(blockRes, nil)

	trRes := utils.TransactionResponse(trExtrinsicIndex1, isSuccess1, fee1, trHash1, time1)
	ic.ProxyClient.On("GetTransactionByHeight", startHeight).Return(trRes, nil)

	evRes := utils.EventResponse(evIds1)
	ic.ProxyClient.On("GetEventByHeight", startHeight).Return(evRes, nil)

	blockRes2 := &blockpb.GetByHeightResponse{}
	e := errors.New("new block error")
	ic.ProxyClient.On("GetBlockByHeight", endHeight).Return(blockRes2, e)

	req := structs.HeightRange{
		StartHeight: startHeight,
		EndHeight:   endHeight,
	}

	var buffer bytes.Buffer
	err = json.NewEncoder(&buffer).Encode(req)
	ic.Require().Nil(err)

	tr := cStructs.TaskRequest{
		Id:      reqID,
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
		if response.Id != reqID || response.Error.Msg == "" {
			continue
		}

		ic.Require().True(response.Final)
		ic.Require().Contains(response.Error.Msg, "Error while getting Transactions with given range: Error while getting transactions: new block error")
		return
	}
}

func (ic *IndexerClientTest) TestGetTransactions_GetTransactionByHeightError() {
	startHeight := uint64(7926)
	endHeight := uint64(7927)
	now := time.Now()
	time2 := now.Add(-500 * time.Second)
	blockHash1 := "0x2326841a64e0a3fff2b4bb760d316cc74b33a8a9480a28ab7e7885acba85e3cf"
	blockHash2 := "316cc74b33a8a9480a28ab7e7885acba0x2326841a64e0a3fff2b4bb760d85e3cf"
	reqID, err := uuid.NewRandom()
	ic.Require().Nil(err)

	blockRes := utils.BlockResponse(int64(startHeight), blockHash1, timestamppb.New(now))
	ic.ProxyClient.On("GetBlockByHeight", startHeight).Return(blockRes, nil)

	blockRes2 := utils.BlockResponse(int64(endHeight), blockHash2, timestamppb.New(time2))
	ic.ProxyClient.On("GetBlockByHeight", endHeight).Return(blockRes2, nil)

	e := errors.New("new transaction error one")
	ic.ProxyClient.On("GetTransactionByHeight", startHeight).Return(&transactionpb.GetByHeightResponse{}, e)

	e2 := errors.New("new transaction error two")
	ic.ProxyClient.On("GetTransactionByHeight", endHeight).Return(&transactionpb.GetByHeightResponse{}, e2)

	req := structs.HeightRange{
		StartHeight: startHeight,
		EndHeight:   endHeight,
	}

	var buffer bytes.Buffer
	err = json.NewEncoder(&buffer).Encode(req)
	ic.Require().Nil(err)

	tr := cStructs.TaskRequest{
		Id:      reqID,
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
		if response.Id != reqID || response.Error.Msg == "" {
			continue
		}

		ic.Require().True(response.Final)

		errMsg := response.Error.Msg
		ic.Require().Contains(errMsg, "Error while getting Transactions with given range: Error while getting transactions")
		ic.Require().Contains(errMsg, "new transaction error one")
		ic.Require().Contains(errMsg, "new transaction error two")
		return
	}
}

func (ic *IndexerClientTest) TestGetTransactions_GetEventByHeightError() {
	startHeight := uint64(7926)
	endHeight := uint64(7927)
	now := time.Now()
	reqID, err := uuid.NewRandom()
	ic.Require().Nil(err)

	blockHash1 := "0x2326841a64e0a3fff2b4bb760d316cc74b33a8a9480a28ab7e7885acba85e3cf"
	trHash1 := "0xe05668b03c6e8bc1dbb61afab01d0515460693e5ade5627461c6298fcf6b8e72"
	trExtrinsicIndex1 := int64(7)
	fee1 := "1000"
	isSuccess1 := true
	time1 := strconv.Itoa(int(now.Unix()))

	time2 := now.Add(-500 * time.Second)
	blockHash2 := "316cc74b33a8a9480a28ab7e7885acba0x2326841a64e0a3fff2b4bb760d85e3cf"
	trHash2 := "5668b03c61afab01d05150xe0460693e56e8bc1dbbade5627461c6298fcf6b8e72"
	trExtrinsicIndex2 := int64(21)
	fee2 := "2000"
	isSuccess2 := false
	time2Str := strconv.Itoa(int(time2.Unix()))

	blockRes := utils.BlockResponse(int64(startHeight), blockHash1, timestamppb.New(now))
	ic.ProxyClient.On("GetBlockByHeight", startHeight).Return(blockRes, nil)

	blockRes2 := utils.BlockResponse(int64(endHeight), blockHash2, timestamppb.New(time2))
	ic.ProxyClient.On("GetBlockByHeight", endHeight).Return(blockRes2, nil)

	trRes := utils.TransactionResponse(trExtrinsicIndex1, isSuccess1, fee1, trHash1, time1)
	ic.ProxyClient.On("GetTransactionByHeight", startHeight).Return(trRes, nil)

	trRes2 := utils.TransactionResponse(trExtrinsicIndex2, isSuccess2, fee2, trHash2, time2Str)
	ic.ProxyClient.On("GetTransactionByHeight", endHeight).Return(trRes2, nil)

	e := errors.New("new event error one")
	ic.ProxyClient.On("GetEventByHeight", startHeight).Return(&eventpb.GetByHeightResponse{}, e)

	e2 := errors.New("new event error two")
	ic.ProxyClient.On("GetEventByHeight", endHeight).Return(&eventpb.GetByHeightResponse{}, e2)

	req := structs.HeightRange{
		StartHeight: startHeight,
		EndHeight:   endHeight,
	}

	var buffer bytes.Buffer
	err = json.NewEncoder(&buffer).Encode(req)
	ic.Require().Nil(err)

	tr := cStructs.TaskRequest{
		Id:      reqID,
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
		if response.Id != reqID || response.Error.Msg == "" {
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
	startHeight := uint64(7926)
	endHeight := uint64(7927)
	now := time.Now()
	reqID, err := uuid.NewRandom()
	ic.Require().Nil(err)

	blockHash1 := "0x2326841a64e0a3fff2b4bb760d316cc74b33a8a9480a28ab7e7885acba85e3cf"
	trHash1 := "0xe05668b03c6e8bc1dbb61afab01d0515460693e5ade5627461c6298fcf6b8e72"
	trExtrinsicIndex1 := int64(7)
	evIds1 := [][2]int64{{1, 3}, {7, 4}}
	fee1 := "1000"
	isSuccess1 := true
	time1 := strconv.Itoa(int(now.Unix()))

	blockRes := utils.BlockResponse(int64(startHeight), blockHash1, timestamppb.New(now))
	ic.ProxyClient.On("GetBlockByHeight", startHeight).Return(blockRes, nil)

	trRes := utils.TransactionResponse(trExtrinsicIndex1, isSuccess1, fee1, trHash1, time1)
	ic.ProxyClient.On("GetTransactionByHeight", startHeight).Return(trRes, nil)

	evRes := utils.EventResponse(evIds1)
	ic.ProxyClient.On("GetEventByHeight", startHeight).Return(evRes, nil)

	blockRes2 := &blockpb.GetByHeightResponse{}
	e := errors.New("new block error")
	ic.ProxyClient.On("GetBlockByHeight", endHeight).Return(blockRes2, e)

	req := structs.HeightRange{
		StartHeight: startHeight,
		EndHeight:   endHeight,
	}

	var buffer bytes.Buffer
	err = json.NewEncoder(&buffer).Encode(req)
	ic.Require().Nil(err)

	tr := cStructs.TaskRequest{
		Id:      reqID,
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
		if response.Id != reqID || response.Error.Msg == "" {
			continue
		}

		ic.Require().True(response.Final)
		ic.Require().Contains(response.Error.Msg, "Error while getting Transactions with given range: Error while getting transactions: new block error")
		return
	}
}

func (ic *IndexerClientTest) validateBlock(block structs.Block, height uint64, hash string, time time.Time) {
	ic.Require().Equal(height, block.Height)
	ic.Require().Equal(hash, block.Hash)
	ic.Require().Equal(time.Unix(), block.Time.Unix())
}

func (ic *IndexerClientTest) validateTransaction(transaction structs.Transaction, height uint64, blockHash, trHash, fee string, isSuccess bool, time time.Time, evIds []string) {
	ic.Require().Equal(height, transaction.Height)
	ic.Require().Equal(blockHash, transaction.BlockHash)
	ic.Require().Equal(trHash, transaction.Hash)
	ic.Require().Equal(strconv.Itoa(int(time.Unix())), transaction.Epoch)
	ic.Require().Equal(chainID, transaction.ChainID)
	ic.Require().Equal(fee, transaction.Fee[0].Text)
	// ic.Require().Equal(time.Unix(), transaction.Time.Unix())
	ic.Require().Equal("0.0.1", transaction.Version)
	ic.Require().Equal(!isSuccess, transaction.HasErrors)

	ic.Require().Len(transaction.Events, len(evIds))
	for _, e := range evIds {
		ic.Require().Equal(e, transaction.Events[0].ID)
	}
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

func (m proxyClientMock) GetEventByHeight(ctx context.Context, height uint64) (*eventpb.GetByHeightResponse, error) {
	args := m.Called(ctx, height)
	return args.Get(0).(*eventpb.GetByHeightResponse), args.Error(1)
}

func (m proxyClientMock) GetTransactionByHeight(ctx context.Context, height uint64) (*transactionpb.GetByHeightResponse, error) {
	args := m.Called(ctx, height)
	return args.Get(0).(*transactionpb.GetByHeightResponse), args.Error(1)
}
