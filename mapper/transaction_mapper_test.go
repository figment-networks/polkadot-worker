package mapper_test

import (
	"math/big"
	"testing"

	"github.com/figment-networks/polkadot-worker/mapper"
	"github.com/figment-networks/polkadot-worker/utils"

	"github.com/figment-networks/indexer-search/structs"
	"github.com/figment-networks/polkadothub-proxy/grpc/decode/decodepb"

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

	DecodeResponse *decodepb.DecodeResponse

	Log *zap.Logger
}

func (tm *TransactionMapperTest) SetupTest() {
	tm.ChainID = "polkadot"
	tm.Currency = "DOT"
	tm.Exp = 10
	tm.Version = "0.0.1"

	div := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(tm.Exp)), nil)
	tm.Divider = new(big.Float).SetFloat64(float64(div.Int64()))

	utils.ReadFile(tm.Suite, "./../utils/decoded.json", &tm.DecodeResponse)

	log, err := zap.NewDevelopment()
	tm.Require().Nil(err)

	tm.Log = log

	tm.TransactionMapper = mapper.NewTransactionMapper(tm.Exp, tm.Log, tm.ChainID, tm.Currency)
}

func (tm *TransactionMapperTest) TestTransactionMapper_TimeParsingError() {
	tm.DecodeResponse.Block.Block.Extrinsics[0].Time = "[object Object]"

	transactions, err := tm.TransactionsMapper(tm.DecodeResponse.Block)

	tm.Require().Nil(transactions)

	tm.Require().NotNil(err)
	tm.Require().Contains(err.Error(), "Could not parse transaction time: strconv.Atoi: parsing \"[object Object]\": invalid syntax")
}

func (tm *TransactionMapperTest) TestTransactionMapper_PartialFeeParsingError() {
	tm.DecodeResponse.Block.Block.Extrinsics[0].PartialFee = "bad"

	transactions, err := tm.TransactionsMapper(tm.DecodeResponse.Block)

	tm.Require().Nil(transactions)

	tm.Require().NotNil(err)
	tm.Require().Contains(err.Error(), "Could not parse transaction partial fee \"bad\"")
}

func (tm *TransactionMapperTest) TestTransactionMapper_TipParsingError() {
	tm.DecodeResponse.Block.Block.Extrinsics[0].PartialFee = ""
	tm.DecodeResponse.Block.Block.Extrinsics[0].Tip = "bad"

	transactions, err := tm.TransactionsMapper(tm.DecodeResponse.Block)

	tm.Require().Nil(transactions)

	tm.Require().NotNil(err)
	tm.Require().Contains(err.Error(), "Could not parse transaction tip \"bad\"")
}

func (tm *TransactionMapperTest) TestTransactionMapper_OK() {
	var expected []*structs.Transaction
	utils.ReadFile(tm.Suite, "./../utils/transactions.json", &expected)

	transactions, err := tm.TransactionsMapper(tm.DecodeResponse.Block)

	tm.Require().Nil(err)

	tm.Require().Len(transactions, 2)

	utils.ValidateTransactions(&tm.Suite, *transactions[0], *expected[0])
}

func TestTransactionMapper(t *testing.T) {
	suite.Run(t, new(TransactionMapperTest))
}
