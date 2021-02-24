package mapper_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/figment-networks/polkadot-worker/mapper"
	"github.com/figment-networks/polkadot-worker/proxy"
	"github.com/figment-networks/polkadot-worker/utils"
	"go.uber.org/zap"

	"github.com/figment-networks/indexing-engine/metrics"
	"github.com/figment-networks/polkadothub-proxy/grpc/block/blockpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/event/eventpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/transaction/transactionpb"

	"github.com/stretchr/testify/suite"
)

type TransactionMapperTest struct {
	suite.Suite

	Args1, Args2           string
	BlockHash              string
	ChainID                string
	Currency               string
	Exp                    int
	EventValues            []utils.EventValues
	Fee1, Fee2             string
	Height                 int64
	IsSuccess1, IsSuccess2 bool
	Method1, Method2       string
	Nonce1, Nonce2         int64
	Section1, Section2     string
	Tip1, Tip2             string
	Time1, Time2           time.Time
	TrHash1, TrHash2       string
	TrIndex1, TrIndex2     int64
	Version                string

	BlockRes       *blockpb.GetByHeightResponse
	EventRes       *eventpb.GetByHeightResponse
	TransactionRes *transactionpb.GetByHeightResponse

	Log *zap.SugaredLogger
}

func (tm *TransactionMapperTest) SetupTest() {
	tm.Args1 = "141H35WTHSyEqEJtj574tKNVDUUvc2pmh88F3YCB5dzpk97g,9000000000"
	tm.Args2 = "1435nBEPwxroPqR2CupS43mP2iVDckz16NEokXRT2j1bE8tH,50000000000"
	tm.ChainID = "chainID"
	tm.Currency = "DOT"
	tm.Exp = 12
	tm.Version = "0.0.1"
	tm.BlockHash = "0x2326841a64e0a3fff2b4bb760d316cc74b33a8a9480a28ab7e7885acba85e3cf"
	tm.Height = int64(120)
	tm.TrIndex1, tm.TrIndex2 = int64(1), int64(2)
	tm.Fee1, tm.Fee2 = "1000", "2000"
	tm.TrHash1 = "0x01d0515460693e5ade5627461c6e05668b03c6e8bc1dbb61afab298fcf6b8e72"
	tm.TrHash2 = "0xe05668b03c6e8bc1dbb61afab01d0515460693e5ade5627461c6298fcf6b8e72"
	tm.IsSuccess1, tm.IsSuccess2 = true, false
	tm.Method1, tm.Method2 = "transfer", "transfer"
	tm.Nonce1, tm.Nonce2 = 432, 2345
	tm.Section1, tm.Section2 = "balances", "balances"
	tm.Tip1, tm.Tip2 = "0", "1000"
	tm.Time1 = time.Now().Add(-500)
	tm.Time2 = time.Now().Add(-900)

	tm.EventValues = []utils.EventValues{{
		Index:          1,
		ExtrinsicIndex: 3,
		EventData: []utils.EventData{{
			Name:  "AccountId",
			Value: "12QVNbQdKKGM1ahx62TPAhc2Gy3G2i8UuizPES3Do1azeDVk",
		}, {
			Name:  "Balance",
			Value: "155000000",
		}},
		Phase:       "applyExtrinsic",
		Method:      "Deposit",
		Section:     "balances",
		Description: `[ Some amount was deposited (e.g. for transaction fees). \[who, deposit\]]`,
	}, {
		Index:          2,
		ExtrinsicIndex: tm.TrIndex1,
		EventData: []utils.EventData{{
			Name:  "AccountId",
			Value: "12QVNbQdKKGM1ahx62TPAhc2Gy3G2i8UuizPES3Do1azeDVk",
		}, {
			Name:  "Balance",
			Value: "155000000",
		}},
		Phase:       "applyExtrinsic",
		Method:      "Deposit",
		Section:     "balances",
		Description: `[ Some amount was deposited (e.g. for transaction fees). \[who, deposit\]]`,
	}, {
		Index:          3,
		ExtrinsicIndex: tm.TrIndex2,
		EventData: []utils.EventData{{
			Name:  "DispatchError",
			Value: `{"Module":{"index":5,"error":4}}`,
		}, {
			Name:  "DispatchInfo",
			Value: `{"weight":218434000,"class":"Normal","paysFee":"Yes"}`,
		}},
		Phase:       "applyExtrinsic",
		Method:      "ExtrinsicFailed",
		Section:     "system",
		Description: `[ An extrinsic failed. \[error, info\]]`,
	}, {
		Index:          4,
		ExtrinsicIndex: tm.TrIndex2,
		EventData: []utils.EventData{{
			Name:  "AccountId",
			Value: "1435nBEPwxroPqR2CupS43mP2iVDckz16NEokXRT2j1bE8tH",
		}, {
			Name:  "Balance",
			Value: "50000000000",
		}},
		Phase:       "applyExtrinsic",
		Method:      "Endowed",
		Section:     "balances",
		Description: `[ An account was created with some free balance. \[account, free_balance\]]`,
	}, {
		Index:          5,
		ExtrinsicIndex: tm.TrIndex2,
		EventData: []utils.EventData{{
			Name:  "AccountId",
			Value: "13SqN5TdZNtpYYyynfWvXBWetnYfTS4TTM63sVpRm8nsvcwe",
		}, {
			Name:  "AccountId",
			Value: "1435nBEPwxroPqR2CupS43mP2iVDckz16NEokXRT2j1bE8tH",
		}, {
			Name:  "Balance",
			Value: "50000000000",
		}},
		Phase:       "applyExtrinsic",
		Method:      "Transfer",
		Section:     "balances",
		Description: `[ Transfer succeeded. \[from, to, value\]]`,
	}}

	tm.BlockRes = utils.BlockResponse(tm.Height, tm.BlockHash, nil)
	tm.EventRes = utils.EventResponse(tm.EventValues)

	tr1 := utils.TransactionResponse(tm.TrIndex1, tm.Nonce1, tm.IsSuccess1, tm.Args1, tm.Fee1, tm.TrHash1, tm.Method1, tm.Section1, tm.Tip1, strconv.Itoa(int(tm.Time1.Unix())))
	tr2 := utils.TransactionResponse(tm.TrIndex2, tm.Nonce2, tm.IsSuccess2, tm.Args2, tm.Fee2, tm.TrHash2, tm.Method2, tm.Section2, tm.Tip2, strconv.Itoa(int(tm.Time2.Unix())))

	tm.TransactionRes = &transactionpb.GetByHeightResponse{
		Transactions: append(append(tr1.Transactions, tr1.Transactions...), tr2.Transactions...),
	}

	conversionDuration := metrics.MustNewHistogramWithTags(metrics.HistogramOptions{})
	proxy.TransactionConversionDuration = conversionDuration.WithLabels("transaction")

	log, err := zap.NewDevelopment()
	tm.Require().Nil(err)

	tm.Log = log.Sugar()
}

func (tm *TransactionMapperTest) TestTransactionMapper_EmptyResponse() {
	transactions, err := mapper.TransactionsMapper(tm.Log, nil, tm.EventRes, tm.TransactionRes, tm.Exp, tm.ChainID, tm.Currency, tm.Version)

	tm.Require().Nil(transactions)
	tm.Require().Nil(err)

	transactions, err = mapper.TransactionsMapper(tm.Log, tm.BlockRes, nil, tm.TransactionRes, tm.Exp, tm.ChainID, tm.Currency, tm.Version)

	tm.Require().Nil(transactions)
	tm.Require().Nil(err)

	transactions, err = mapper.TransactionsMapper(tm.Log, tm.BlockRes, tm.EventRes, nil, tm.Exp, tm.ChainID, tm.Currency, tm.Version)

	tm.Require().Nil(transactions)
	tm.Require().Nil(err)
}

func (tm *TransactionMapperTest) TestTransactionMapper_DescriptionParsingError() {
	tm.EventRes.Events[0].Description = "bad description"

	transactions, err := mapper.TransactionsMapper(tm.Log, tm.BlockRes, tm.EventRes, tm.TransactionRes, tm.Exp, tm.ChainID, tm.Currency, tm.Version)

	tm.Require().Nil(transactions)

	tm.Require().NotNil(err)
	tm.Require().Contains(err.Error(), "Could not get values from description \"bad description\"")
}

func (tm *TransactionMapperTest) TestTransactionMapper_TimeParsingError() {
	tm.TransactionRes.Transactions[0].Time = "[object Object]"

	transactions, err := mapper.TransactionsMapper(tm.Log, tm.BlockRes, tm.EventRes, tm.TransactionRes, tm.Exp, tm.ChainID, tm.Currency, tm.Version)

	tm.Require().Nil(transactions)

	tm.Require().NotNil(err)
	tm.Require().Contains(err.Error(), "Could not parse transaction time: strconv.Atoi: parsing \"[object Object]\": invalid syntax")
}

func (tm *TransactionMapperTest) TestTransactionMapper_PartialFeeParsingError() {
	tm.TransactionRes.Transactions[0].PartialFee = "bad"

	transactions, err := mapper.TransactionsMapper(tm.Log, tm.BlockRes, tm.EventRes, tm.TransactionRes, tm.Exp, tm.ChainID, tm.Currency, tm.Version)

	tm.Require().Nil(transactions)

	tm.Require().NotNil(err)
	tm.Require().Contains(err.Error(), "Could not parse transaction partial fee \"bad\"")
}

func (tm *TransactionMapperTest) TestTransactionMapper_TipParsingError() {
	tm.TransactionRes.Transactions[0].Tip = "bad"

	transactions, err := mapper.TransactionsMapper(tm.Log, tm.BlockRes, tm.EventRes, tm.TransactionRes, tm.Exp, tm.ChainID, tm.Currency, tm.Version)

	tm.Require().Nil(transactions)

	tm.Require().NotNil(err)
	tm.Require().Contains(err.Error(), "Could not parse transaction tip \"bad\"")
}

func (tm *TransactionMapperTest) TestTransactionMapper_OK() {
	transactions, err := mapper.TransactionsMapper(tm.Log, tm.BlockRes, tm.EventRes, tm.TransactionRes, tm.Exp, tm.ChainID, tm.Currency, tm.Version)

	tm.Require().Nil(err)

	tm.Require().Len(transactions, 2)

	for _, tr := range transactions {
		switch tr.Hash {
		case tm.TrHash1:
			// additional1 := []string{`name:"AccountId" value:"12QVNbQdKKGM1ahx62TPAhc2Gy3G2i8UuizPES3Do1azeDVk"`, `name:"Balance" value:"155000000"`}
			accountID := ""
			senderID := ""
			recipientID := "12QVNbQdKKGM1ahx62TPAhc2Gy3G2i8UuizPES3Do1azeDVk"
			utils.ValidateTransaction(&tm.Suite, *tr, tm.Time1, []utils.EventValues{tm.EventValues[1]}, nil, tm.Exp,
				uint64(tm.Height), tm.IsSuccess1, tm.BlockHash, tm.ChainID, tm.Currency, tm.TrHash1, tm.Fee1, "0.000000001DOT", "155000000", "0.000155DOT", accountID, senderID, recipientID)

		case tm.TrHash2:
			// additional2 := []string{`name:"DispatchError" value:"{\"Module\":{\"index\":5,\"error\":4}}"`, `name:"DispatchInfo" value:"{\"weight\":218434000,\"class\":\"Normal\",\"paysFee\":\"Yes\"}"`}
			accountID := "1435nBEPwxroPqR2CupS43mP2iVDckz16NEokXRT2j1bE8tH"
			senderID := "13SqN5TdZNtpYYyynfWvXBWetnYfTS4TTM63sVpRm8nsvcwe"
			recipientID := "1435nBEPwxroPqR2CupS43mP2iVDckz16NEokXRT2j1bE8tH"
			utils.ValidateTransaction(&tm.Suite, *tr, tm.Time2, tm.EventValues[2:], nil, tm.Exp,
				uint64(tm.Height), tm.IsSuccess2, tm.BlockHash, tm.ChainID, tm.Currency, tm.TrHash2, "3000", "0.000000003DOT", "50000000000", "0.05DOT", accountID, senderID, recipientID)
		}
	}

}

func TestTransactionMapper(t *testing.T) {
	suite.Run(t, new(TransactionMapperTest))
}
