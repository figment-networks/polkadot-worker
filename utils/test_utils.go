package utils

import (
	"math/big"
	"strconv"
	"time"

	"github.com/figment-networks/indexer-manager/structs"
	"github.com/figment-networks/polkadothub-proxy/grpc/block/blockpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/chain/chainpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/event/eventpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/transaction/transactionpb"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type BlockResp struct {
	Hash   string
	Height int64
	Time   *timestamppb.Timestamp
}

func BlockResponse(resp BlockResp) *blockpb.GetByHeightResponse {
	return &blockpb.GetByHeightResponse{
		Block: &blockpb.Block{
			BlockHash: resp.Hash,
			Header: &blockpb.Header{
				Height: resp.Height,
				Time:   resp.Time,
			},
		},
	}
}

type EventsResp struct {
	Index          int64
	ExtrinsicIndex int64
	Description    string
	Method         string
	Phase          string
	Section        string
	EventData      []EventData

	AccountsID  []string
	Additional  [][]string
	Amount      string
	AmountText  string
	SenderID    string
	RecipientID string
}

type EventData struct {
	Name  string
	Value string
}

func EventsResponse(resp []EventsResp) *eventpb.GetByHeightResponse {
	evts := make([]*eventpb.Event, len(resp))

	for i, e := range resp {
		data := make([]*eventpb.EventData, len(e.EventData))

		for j, d := range e.EventData {
			data[j] = &eventpb.EventData{
				Name:  d.Name,
				Value: d.Value,
			}
		}

		evts[i] = &eventpb.Event{
			Index:          e.Index,
			ExtrinsicIndex: e.ExtrinsicIndex,
			Data:           data,
			Method:         e.Method,
			Phase:          e.Phase,
			Description:    e.Description,
			Section:        e.Section,
		}
	}

	return &eventpb.GetByHeightResponse{Events: evts}
}

type MetaResp struct {
	Era int64
}

func MetaResponse(resp MetaResp) *chainpb.GetMetaByHeightResponse {
	return &chainpb.GetMetaByHeightResponse{
		Era: resp.Era,
	}
}

type TransactionsResp struct {
	Index     int64
	Args      string
	Fee       string
	Hash      string
	IsSuccess bool
	Raw       string
	Nonce     int64
	Method    string
	Section   string
	Tip       string
	Time      string

	FeeAmount    string
	FeeAmountTxt string
}

func TransactionsResponse(resp TransactionsResp) *transactionpb.GetByHeightResponse {
	return &transactionpb.GetByHeightResponse{
		Transactions: []*transactionpb.Transaction{{
			ExtrinsicIndex: resp.Index,
			Args:           resp.Args,
			Hash:           resp.Hash,
			IsSuccess:      resp.IsSuccess,
			PartialFee:     resp.Fee,
			Raw:            resp.Raw,
			Nonce:          resp.Nonce,
			Method:         resp.Method,
			Section:        resp.Section,
			Tip:            resp.Tip,
			Time:           resp.Time,
		}},
	}
}

func ValidateTransactions(sut *suite.Suite, transaction structs.Transaction, bResp BlockResp, trResp []TransactionsResp, evResp [][]EventsResp, mResp MetaResp, chainID, currency string, exp int32) {
	sut.Require().Equal(bResp.Hash, transaction.BlockHash)
	sut.Require().EqualValues(bResp.Height, transaction.Height)
	sut.Require().EqualValues(bResp.Height, transaction.Height)

	for i, t := range trResp {
		sut.Require().Equal(t.Hash, transaction.Hash)
		sut.Require().Equal(chainID, transaction.ChainID)
		sut.Require().Equal(t.Time, strconv.Itoa(int(transaction.Time.Unix())))
		sut.Require().Equal("0.0.1", transaction.Version)
		sut.Require().Equal(!t.IsSuccess, transaction.HasErrors)
		sut.Require().Equal(strconv.Itoa(int(mResp.Era)), transaction.Epoch)

		feeNumeric, ok := new(big.Int).SetString(t.FeeAmount, 10)
		sut.Require().True(ok)
		sut.Require().Equal(feeNumeric, transaction.Fee[0].Numeric)
		sut.Require().Equal(t.FeeAmountTxt, transaction.Fee[0].Text)
		sut.Require().Equal(currency, transaction.Fee[0].Currency)
		sut.Require().EqualValues(exp, transaction.Fee[0].Exp)

		sut.Require().Len(transaction.Events, len(evResp[i]))
		for i, e := range evResp[i] {
			sut.Require().Equal(strconv.Itoa(int(e.Index)), transaction.Events[i].ID)
			// sub.Require().Equal(kind, transaction.Events[i].Kind)
			// sub.Require().Equal(action, transaction.Events[i].Sub[0].Action)

			expectedType := e.Method
			for _, d := range e.EventData {
				if d.Name == "DispatchError" {
					expectedType = "error"
				}
			}

	validateAmount(sut, &transaction.Fee[0], int(exp), trResp.FeeAmount, trResp.FeeAmountTxt, currency)

	sut.Require().Len(transaction.Events, len(evResp))
	for i, e := range evResp {
		sut.Require().Equal(strconv.Itoa(int(e.Index)), transaction.Events[i].ID)

		expectedType := e.Method
		for _, d := range e.EventData {
			if d.Name == "DispatchError" {
				expectedType = "error"
			}
		}

		event := transaction.Events[i].Sub[0]
		sut.Require().Equal(e.Section, event.Module)
		sut.Require().Equal([]string{expectedType}, event.Type)
		sut.Require().Equal(trResp.Time, strconv.Itoa(int(event.Completion.Unix())))

		if a, ok := event.Amount["0"]; ok {
			validateAmount(sut, &a, int(exp), e.Amount, e.AmountText, currency)
		}

		if accounts, ok := event.Node["versions"]; ok {
			for i, account := range accounts {
				sut.Require().Equal(e.AccountsID[i], account.ID)
			}
		}

		if account, ok := event.Node["sender"]; ok {
			sut.Require().Equal(e.SenderID, account[0].ID)
		}

		if account, ok := event.Node["recipient"]; ok {
			sut.Require().Equal(e.RecipientID, account[0].ID)

			transfers := event.Transfers[e.RecipientID]
			sut.Require().Equal(e.RecipientID, transfers[0].Account.ID)

			validateAmount(sut, &transfers[0].Amounts[0], int(exp), e.Amount, e.AmountText, currency)
		}

		for _, aa := range e.Additional {
			for _, a := range aa {
				founded := false
				for j := 0; j < len(e.Additional); j++ {
					if sut.Contains(event.Additional["attributes"][j], a) {
						founded = true
					}
				}
				sut.Require().True(founded)
			}
		}
	}
}

func validateAmount(sut *suite.Suite, amount *structs.TransactionAmount, exp int, a, aTxt, currency string) {
	if amount == nil {
		return
	}

	n, ok := new(big.Int).SetString(a, 10)
	sut.Require().True(ok)

	sut.Require().Equal(n, amount.Numeric)
	sut.Require().Equal(aTxt, amount.Text)
	sut.Require().Equal(currency, amount.Currency)
	sut.Require().EqualValues(exp, amount.Exp)
}

func GetTransactionsResponses(height [2]uint64) [][]TransactionsResp {
	now := time.Now()

	return []TransactionsResp{{
		Index:     1,
		Args:      "",
		Fee:       "1000",
		Hash:      "0xd922a8a95e1dcf62375026ffd412c28f842854462296695d935c296f5107153f",
		IsSuccess: true,
		Raw:       `{"isSigned":true,"method":{"args":[{"Id":"14coxGrE4uD8ZMascmAPXhvggwnp8bgdW2fFVWMZSqJEFxCV"},"10.0000 mDOT (old)"],"method":"transferKeepAlive","section":"balances"},"era":{"MortalEra":{"period":"64","phase":"18"}},"nonce":"123","signature":"0xa8762de3dbfafab6cd98129dc82b1cfbcc772d62d949a0d1d065bc28a1774a090f8dde927970876c30d618f8b299422f9f1eb1a4ecb3b1eaba420c8005c04585","signer":{"Id":"14rX5P237x97oEdNNkQurUCf9myK6T6fonYeqpyE5BcLfKqh"},"tip":"1"}`,
		Nonce:     123,
		Method:    "transferKeepAlive",
		Section:   "balances",
		Tip:       "1",
		Time:      strconv.Itoa(int(now.Add(-100 * time.Second).Unix())),

		FeeAmount:    "1001",
		FeeAmountTxt: "0.000000001001DOT",
	}, {
		Index:     1,
		Args:      "",
		Fee:       "156000000",
		Hash:      "0xf59d6c5642ddec857fdba05544332340e25a3a73cb1b74c78f3b79bdbed8b6fe",
		IsSuccess: true,
		Raw:       `{"isSigned":true,"method":{"args":[{"Id":"14coxGrE4uD8ZMascmAPXhvggwnp8bgdW2fFVWMZSqJEFxCV"},"10.0000 mDOT (old)"],"method":"transferKeepAlive","section":"balances"},"era":{"MortalEra":{"period":"64","phase":"18"}},"nonce":"0","signature":"0xa8762de3dbfafab6cd98129dc82b1cfbcc772d62d949a0d1d065bc28a1774a090f8dde927970876c30d618f8b299422f9f1eb1a4ecb3b1eaba420c8005c04585","signer":{"Id":"14rX5P237x97oEdNNkQurUCf9myK6T6fonYeqpyE5BcLfKqh"},"tip":"0"}`,
		Nonce:     123,
		Method:    "transfer",
		Section:   "balances",
		Tip:       "0",
		Time:      strconv.Itoa(int(now.Add(-100 * time.Second).Unix())),

		FeeAmount:    "156000000",
		FeeAmountTxt: "0.000156DOT",
	}}
}

func GetBlocksResponses(height [2]uint64) []BlockResp {
	now := time.Now()
	return []BlockResp{{
		Hash:   "0x9291d0465056465420ee87ce768527b320de496a6b6a75f84c14622043d6d413",
		Height: int64(height[0]),
		Time:   timestamppb.New(now.Add(-100 * time.Second)),
	}, {
		Hash:   "0x502f2a74beb519186d85cd3ed7f6dea30b821fad95e4820b2e220e91175f7aff",
		Height: int64(height[1]),
		Time:   timestamppb.New(now),
	}}
}

func GetEventsResponses(height [2]uint64) [][]EventsResp {
	return [][]EventsResp{{{
		Index:          0,
		ExtrinsicIndex: 0,
		Description:    "[ An extrinsic completed successfully.]",
		Method:         "ExtrinsicSuccess",
		Phase:          "applyExtrinsic",
		Section:        "system",
		EventData: []EventData{{
			Name:  "DispatchInfo",
			Value: "{\"weight\":161397000,\"class\":\"Mandatory\",\"paysFee\":\"Yes\"}",
		}},
	}, {
		Index:          1,
		ExtrinsicIndex: 1,
		Description:    "[ A new \\[account\\] was created.]",
		Method:         "NewAccount",
		Phase:          "applyExtrinsic",
		Section:        "system",
		EventData: []EventData{{
			Name:  "AccountId",
			Value: "14coxGrE4uD8ZMascmAPXhvggwnp8bgdW2fFVWMZSqJEFxCV",
		}},

		AccountsID: []string{"14coxGrE4uD8ZMascmAPXhvggwnp8bgdW2fFVWMZSqJEFxCV"},
	}, {
		Index:          2,
		ExtrinsicIndex: 1,
		Description:    "[ An account was created with some free balance. \\[account, free_balance\\]]",
		Method:         "Endowed",
		Phase:          "applyExtrinsic",
		Section:        "balances",
		EventData: []EventData{{
			Name:  "AccountId",
			Value: "14coxGrE4uD8ZMascmAPXhvggwnp8bgdW2fFVWMZSqJEFxCV",
		}, {
			Name:  "Balance",
			Value: "10000000000",
		}},

		AccountsID: []string{"14coxGrE4uD8ZMascmAPXhvggwnp8bgdW2fFVWMZSqJEFxCV"},
		Amount:     "10000000000",
		AmountText: "0.01DOT",
	}, {
		Index:          3,
		ExtrinsicIndex: 1,
		Description:    "[ Transfer succeeded. \\[from, to, value\\]]",
		Method:         "Transfer",
		Phase:          "applyExtrinsic",
		Section:        "balances",
		EventData: []EventData{{
			Name:  "AccountId",
			Value: "14rX5P237x97oEdNNkQurUCf9myK6T6fonYeqpyE5BcLfKqh",
		}, {
			Name:  "AccountId",
			Value: "14coxGrE4uD8ZMascmAPXhvggwnp8bgdW2fFVWMZSqJEFxCV",
		}, {
			Name:  "Balance",
			Value: "10000000000",
		}},

		SenderID:    "14rX5P237x97oEdNNkQurUCf9myK6T6fonYeqpyE5BcLfKqh",
		RecipientID: "14coxGrE4uD8ZMascmAPXhvggwnp8bgdW2fFVWMZSqJEFxCV",
		Amount:      "10000000000",
		AmountText:  "0.01DOT",
	}, {
		Index:          4,
		ExtrinsicIndex: 1,
		Description:    "[ Some amount was deposited (e.g. for transaction fees). \\[who, deposit\\]]",
		Method:         "Deposit",
		Phase:          "applyExtrinsic",
		Section:        "balances",
		EventData: []EventData{{
			Name:  "AccountId",
			Value: "15AcyKihrmGs9RD4AHUwRvv6LkhbeDyGH3GVADp1Biv4bfFv",
		}, {
			Name:  "Balance",
			Value: "156000000",
		}},

		RecipientID: "15AcyKihrmGs9RD4AHUwRvv6LkhbeDyGH3GVADp1Biv4bfFv",
		Amount:      "156000000",
		AmountText:  "0.000156DOT",
	}, {
		Index:          5,
		ExtrinsicIndex: 1,
		Description:    "[ An extrinsic completed successfully. \\[info\\]]",
		Method:         "ExtrinsicSuccess",
		Phase:          "applyExtrinsic",
		Section:        "system",
		EventData: []EventData{{
			Name:  "DispatchInfo",
			Value: "{\"weight\":189060000,\"class\":\"Normal\",\"paysFee\":\"Yes\"}",
		}},
	}}, {{
		Index:          0,
		ExtrinsicIndex: 0,
		Description:    "[ An extrinsic completed successfully. \\[info\\]]",
		Method:         "ExtrinsicSuccess",
		Phase:          "applyExtrinsic",
		Section:        "system",
		EventData: []EventData{{
			Name:  "DispatchInfo",
			Value: "{\"weight\":161397000,\"class\":\"Mandatory\",\"paysFee\":\"Yes\"}",
		}},
	}, {
		Index:          1,
		ExtrinsicIndex: 1,
		Description:    "[ An account was removed whose balance was non-zero but below ExistentialDeposit,,  resulting in an outright loss. \\[account, balance\\]]",
		Method:         "DustLost",
		Phase:          "applyExtrinsic",
		Section:        "balances",
		EventData: []EventData{{
			Name:  "AccountId",
			Value: "14DzyWx1Xq7ZqEjXMWJp6jRUq34hAXJHuj8wts3kRvikEvo5",
		}, {
			Name:  "Balance",
			Value: "10000000",
		}},

		AccountsID: []string{"14DzyWx1Xq7ZqEjXMWJp6jRUq34hAXJHuj8wts3kRvikEvo5"},
		Amount:     "10000000",
		AmountText: "0.00001DOT",
	}}}
}

func GetMetaResponses(height [2]uint64) []MetaResp {
	return []MetaResp{{
		Era: 352,
	}, {
		Era: 231,
	}}
}
