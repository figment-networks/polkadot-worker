package mapper_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/figment-networks/indexing-engine/metrics"
	"github.com/figment-networks/polkadot-worker/mapper"
	"github.com/figment-networks/polkadot-worker/proxy"
	"github.com/figment-networks/polkadot-worker/utils"

	"github.com/figment-networks/polkadothub-proxy/grpc/block/blockpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/event/eventpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/transaction/transactionpb"

	"github.com/stretchr/testify/suite"
)

type TransactionMapTest struct {
	suite.Suite

	BlockHash              string
	ChainID                string
	EvIds                  [][2]int64
	Fee1, Fee2             string
	Height                 int64
	IsSuccess1, IsSuccess2 bool
	Time1, Time2           string
	TrHash1, TrHash2       string
	TrIndex1, TrIndex2     int64
	Version                string

	BlockRes       *blockpb.GetByHeightResponse
	EventRes       *eventpb.GetByHeightResponse
	TransactionRes *transactionpb.GetByHeightResponse
}

func (tm *TransactionMapTest) SetupTest() {
	tm.ChainID = "chainID"
	tm.Version = "0.0.1"
	tm.BlockHash = "0x2326841a64e0a3fff2b4bb760d316cc74b33a8a9480a28ab7e7885acba85e3cf"
	tm.Height = int64(120)
	tm.EvIds = [][2]int64{{1, 3}, {2, 2}}
	tm.TrIndex1, tm.TrIndex1 = int64(1), int64(2)
	tm.Fee1, tm.Fee2 = "1000", "2000"
	tm.TrHash1 = "0x01d0515460693e5ade5627461c6e05668b03c6e8bc1dbb61afab298fcf6b8e72"
	tm.TrHash2 = "0xe05668b03c6e8bc1dbb61afab01d0515460693e5ade5627461c6298fcf6b8e72"
	tm.IsSuccess1, tm.IsSuccess2 = true, false
	tm.Time1 = strconv.Itoa(int(time.Now().Add(-500).Unix()))
	tm.Time2 = strconv.Itoa(int(time.Now().Add(-900).Unix()))

	tm.BlockRes = utils.BlockResponse(tm.Height, tm.BlockHash, nil)
	tm.EventRes = utils.EventResponse(tm.EvIds)

	tr1 := utils.TransactionResponse(tm.TrIndex1, tm.IsSuccess1, tm.Fee1, tm.TrHash1, tm.Time1)
	tr2 := utils.TransactionResponse(tm.TrIndex2, tm.IsSuccess2, tm.Fee2, tm.TrHash2, tm.Time2)

	tm.TransactionRes = &transactionpb.GetByHeightResponse{
		Transactions: append(append(tr1.Transactions, tr1.Transactions...), tr2.Transactions...),
	}

	conversionDuration := metrics.MustNewHistogramWithTags(metrics.HistogramOptions{})
	proxy.TransactionConversionDuration = conversionDuration.WithLabels("transaction")
}

func (tm *TransactionMapTest) TestTransactionMap_EmptyResponse() {
	transaction, err := mapper.TransactionMapper(nil, tm.EventRes, tm.TransactionRes, tm.ChainID, tm.Version)

	tm.Require().Nil(transaction)
	tm.Require().Nil(err)

	transaction, err = mapper.TransactionMapper(tm.BlockRes, nil, tm.TransactionRes, tm.ChainID, tm.Version)

	tm.Require().Nil(transaction)
	tm.Require().Nil(err)

	transaction, err = mapper.TransactionMapper(tm.BlockRes, tm.EventRes, nil, tm.ChainID, tm.Version)

	tm.Require().Nil(transaction)
	tm.Require().Nil(err)
}

func (tm *TransactionMapTest) TestTransactionMap_TimeParsingError() {
	tm.TransactionRes.Transactions[0].Time = "[object Object]"

	transaction, err := mapper.TransactionMapper(tm.BlockRes, tm.EventRes, tm.TransactionRes, tm.ChainID, tm.Version)

	tm.Require().Nil(transaction)

	tm.Require().NotNil(err)
	tm.Require().Contains(err.Error(), "Could not parse transaction time: strconv.Atoi: parsing \"[object Object]\": invalid syntax")
}

func (tm *TransactionMapTest) TestTransactionMap_OK() {
	transaction, err := mapper.TransactionMapper(tm.BlockRes, tm.EventRes, tm.TransactionRes, tm.ChainID, tm.Version)

	tm.Require().Nil(err)

	tm.Require().Len(transaction, 2)

	tm.Require().Equal(tm.BlockHash, transaction[0].BlockHash)
	tm.Require().Equal(tm.ChainID, transaction[0].ChainID)
	tm.Require().EqualValues(tm.Height, transaction[0].Height)

	tm.Require().Equal(tm.TrHash1, transaction[0].Hash)
	tm.Require().Equal(tm.Time1, transaction[0].Epoch)

	timeInt1, _ := strconv.Atoi(tm.Time1)
	tm.Require().Equal(time.Unix(int64(timeInt1), 0), transaction[0].Time)

	tm.Require().Equal(tm.Fee1, transaction[0].Fee[0].Text)
	tm.Require().Equal("0.0.1", transaction[0].Version)
	tm.Require().Equal(!tm.IsSuccess1, transaction[0].HasErrors)

	tm.Require().Equal(tm.TrHash2, transaction[1].Hash)
	tm.Require().Equal(tm.Time2, transaction[1].Epoch)

	timeInt2, _ := strconv.Atoi(tm.Time2)
	tm.Require().Equal(time.Unix(int64(timeInt2), 0), transaction[1].Time)

	tm.Require().Equal(tm.Fee2, transaction[1].Fee[0].Text)
	tm.Require().Equal("0.0.1", transaction[1].Version)
	tm.Require().Equal(!tm.IsSuccess2, transaction[1].HasErrors)
}

func TestTransactionMap(t *testing.T) {
	suite.Run(t, new(TransactionMapTest))
}
