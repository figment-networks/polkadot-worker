package mapper_test

import (
	"math/big"
	"testing"

	"github.com/figment-networks/polkadot-worker/mapper"
	"github.com/figment-networks/polkadot-worker/utils"

	"github.com/figment-networks/polkadothub-proxy/grpc/block/blockpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/chain/chainpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/event/eventpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/transaction/transactionpb"

	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

type TransactionMapperTest struct {
	suite.Suite
	*mapper.TransactionMapper

	ChainID  string
	Currency string
	Divider  *big.Float
	Exp      int
	Version  string

	Blocks       []utils.BlockResp
	Events       [][]utils.EventsResp
	Metas        []utils.MetaResp
	Transactions []utils.TransactionsResp

	BlockResponse        *blockpb.GetByHeightResponse
	EventsResponse       *eventpb.GetByHeightResponse
	MetaResponse         *chainpb.GetMetaByHeightResponse
	TransactionsResponse *transactionpb.GetByHeightResponse

	Log *zap.Logger
}

func (tm *TransactionMapperTest) SetupTest() {
	//conversionDuration := metrics.MustNewHistogramWithTags(metrics.HistogramOptions{})
	//transactionConversionDuration = conversionDuration.WithLabels("transaction")

	tm.ChainID = "Polkadot"
	tm.Currency = "DOT"
	tm.Exp = 12
	tm.Version = "0.0.1"

	height := [2]uint64{123, 321}

	div := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(tm.Exp)), nil)
	tm.Divider = new(big.Float).SetFloat64(float64(div.Int64()))

	tm.Blocks = utils.GetBlocksResponses(height)
	tm.Events = utils.GetEventsResponses(height)
	tm.Metas = utils.GetMetaResponses(height)
	tm.Transactions = utils.GetTransactionsResponses(height)

	tm.BlockResponse = utils.BlockResponse(tm.Blocks[0])
	tm.EventsResponse = utils.EventsResponse(tm.Events[0])
	tm.MetaResponse = utils.MetaResponse(tm.Metas[0])
	tm.TransactionsResponse = utils.TransactionsResponse(tm.Transactions[0])

	log, err := zap.NewDevelopment()
	tm.Require().Nil(err)

	tm.Log = log

	tm.TransactionMapper = mapper.NewTransactionMapper(tm.Exp, tm.ChainID, tm.Currency)
}

func (tm *TransactionMapperTest) TestTransactionMapper_EmptyResponse() {
	transactions, err := tm.TransactionsMapper(tm.Log, nil, tm.EventsResponse, tm.MetaResponse, tm.TransactionsResponse)

	tm.Require().Nil(transactions)
	tm.Require().NotNil(err)
	tm.Require().Contains("One of required proxy response is missing", err.Error())

	transactions, err = tm.TransactionsMapper(tm.Log, tm.BlockResponse, nil, tm.MetaResponse, tm.TransactionsResponse)

	tm.Require().Nil(transactions)
	tm.Require().NotNil(err)
	tm.Require().Contains("One of required proxy response is missing", err.Error())

	transactions, err = tm.TransactionsMapper(tm.Log, tm.BlockResponse, tm.EventsResponse, nil, tm.TransactionsResponse)

	tm.Require().Nil(transactions)
	tm.Require().NotNil(err)
	tm.Require().Contains("One of required proxy response is missing", err.Error())

	transactions, err = tm.TransactionsMapper(tm.Log, tm.BlockResponse, tm.EventsResponse, tm.MetaResponse, nil)

	tm.Require().Nil(transactions)
	tm.Require().NotNil(err)
	tm.Require().Contains("One of required proxy response is missing", err.Error())
}

func (tm *TransactionMapperTest) TestTransactionMapper_TimeParsingError() {
	tm.TransactionsResponse.Transactions[0].Time = "[object Object]"

	transactions, err := tm.TransactionsMapper(tm.Log, tm.BlockResponse, tm.EventsResponse, tm.MetaResponse, tm.TransactionsResponse)

	tm.Require().Nil(transactions)

	tm.Require().NotNil(err)
	tm.Require().Contains(err.Error(), "Could not parse transaction time: strconv.Atoi: parsing \"[object Object]\": invalid syntax")
}

func (tm *TransactionMapperTest) TestTransactionMapper_PartialFeeParsingError() {
	tm.TransactionsResponse.Transactions[0].PartialFee = "bad"

	transactions, err := tm.TransactionsMapper(tm.Log, tm.BlockResponse, tm.EventsResponse, tm.MetaResponse, tm.TransactionsResponse)

	tm.Require().Nil(transactions)

	tm.Require().NotNil(err)
	tm.Require().Contains(err.Error(), "Could not parse transaction partial fee \"bad\"")
}

func (tm *TransactionMapperTest) TestTransactionMapper_TipParsingError() {
	tm.TransactionsResponse.Transactions[0].Tip = "bad"

	transactions, err := tm.TransactionsMapper(tm.Log, tm.BlockResponse, tm.EventsResponse, tm.MetaResponse, tm.TransactionsResponse)

	tm.Require().Nil(transactions)

	tm.Require().NotNil(err)
	tm.Require().Contains(err.Error(), "Could not parse transaction tip \"bad\"")
}

func (tm *TransactionMapperTest) TestTransactionMapper_OK() {
	transactions, err := tm.TransactionsMapper(tm.Log, tm.BlockResponse, tm.EventsResponse, tm.MetaResponse, tm.TransactionsResponse)

	tm.Require().Nil(err)

	tm.Require().Len(transactions, 1)

	expectedEvents := tm.Events[0][1:]
	utils.ValidateTransactions(&tm.Suite, *transactions[0], tm.Blocks[0], tm.Transactions[0], expectedEvents, tm.Metas[0], tm.ChainID, tm.Currency, int32(tm.Exp))
}

func TestTransactionMapper(t *testing.T) {
	suite.Run(t, new(TransactionMapperTest))
}
