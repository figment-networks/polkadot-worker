package utils

import (
	"math/big"
	"strconv"

	"github.com/figment-networks/indexer-manager/structs"
	"github.com/figment-networks/polkadothub-proxy/grpc/block/blockpb"
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

type EventsRes struct {
	Index          int64
	ExtrinsicIndex int64
	Description    string
	Method         string
	Phase          string
	Section        string
	EventData      []EventData

	AccountID   string
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

func EventsResponse(resp []EventsRes) *eventpb.GetByHeightResponse {
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

func TransactionsResponse(resp []TransactionsResp) *transactionpb.GetByHeightResponse {
	transactions := make([]*transactionpb.Transaction, len(resp))
	for i, t := range resp {
		transactions[i] = &transactionpb.Transaction{
			ExtrinsicIndex: t.Index,
			Args:           t.Args,
			Hash:           t.Hash,
			IsSuccess:      t.IsSuccess,
			PartialFee:     t.Fee,
			// Raw:            t.Raw,
			Nonce:   t.Nonce,
			Method:  t.Method,
			Section: t.Section,
			Tip:     t.Tip,
			Time:    t.Time,
		}
	}
	return &transactionpb.GetByHeightResponse{
		Transactions: transactions,
	}
}

func ValidateTransactions(sut *suite.Suite, transaction structs.Transaction, bResp BlockResp, trResp []TransactionsResp, evResp [][]EventsRes, chainID, currency string, exp int32) {
	sut.Require().Equal(bResp.Hash, transaction.BlockHash)
	sut.Require().EqualValues(bResp.Height, transaction.Height)

	for i, t := range trResp {
		sut.Require().Equal(t.Hash, transaction.Hash)
		sut.Require().Equal(chainID, transaction.ChainID)
		sut.Require().Equal(t.Time, strconv.Itoa(int(transaction.Time.Unix())))
		sut.Require().Equal("0.0.1", transaction.Version)
		sut.Require().Equal(!t.IsSuccess, transaction.HasErrors)

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

			event := transaction.Events[i].Sub[0]
			sut.Require().Equal(e.Section, event.Module)
			sut.Require().Equal([]string{expectedType}, event.Type)
			sut.Require().Equal(t.Time, strconv.Itoa(int(event.Completion.Unix())))

			if a, ok := event.Amount["0"]; ok {
				validateAmount(sut, &a, int(exp), e.Amount, e.AmountText, currency)
			}

			if account, ok := event.Node["versions"]; ok {
				sut.Require().Equal(e.AccountID, account[0].ID)
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
