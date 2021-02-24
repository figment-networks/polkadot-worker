package utils

import (
	"math/big"
	"strconv"
	"time"

	"github.com/figment-networks/indexer-manager/structs"
	"github.com/figment-networks/polkadothub-proxy/grpc/block/blockpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/event/eventpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/transaction/transactionpb"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func BlockResponse(height int64, hash string, time *timestamppb.Timestamp) *blockpb.GetByHeightResponse {
	return &blockpb.GetByHeightResponse{
		Block: &blockpb.Block{
			BlockHash: hash,
			Header: &blockpb.Header{
				Height: height,
				Time:   time,
			},
		},
	}
}

type EventValues struct {
	Index          int64
	ExtrinsicIndex int64
	EventData      []EventData
	Method         string
	Phase          string
	Description    string
	Section        string
}

type EventData struct {
	Name  string
	Value string
}

func EventResponse(ev []EventValues) *eventpb.GetByHeightResponse {
	evts := make([]*eventpb.Event, len(ev))
	for i, e := range ev {

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

func TransactionResponse(index, nonce int64, isSuccess bool, args, fee, hash, method, section, tip, time string) *transactionpb.GetByHeightResponse {
	return &transactionpb.GetByHeightResponse{
		Transactions: []*transactionpb.Transaction{{
			Args:           args,
			ExtrinsicIndex: index,
			PartialFee:     fee,
			Hash:           hash,
			Method:         method,
			Nonce:          nonce,
			// Raw:            raw,
			Section:   section,
			IsSuccess: isSuccess,
			Tip:       tip,
			Time:      time,
		}},
	}
}

func ValidateTransaction(sut *suite.Suite, transaction structs.Transaction, time time.Time, ev []EventValues, additional []string, exp int, height uint64, isSuccess bool, blockHash, chainID, currency, trHash, fee, feeTxt, amount, amountTxt, accountID, senderID, recipientID string) {
	sut.Require().Equal(height, transaction.Height)
	sut.Require().Equal(blockHash, transaction.BlockHash)
	sut.Require().Equal(trHash, transaction.Hash)
	sut.Require().Equal(chainID, transaction.ChainID)
	sut.Require().Equal(time.Unix(), transaction.Time.Unix())
	sut.Require().Equal("0.0.1", transaction.Version)
	sut.Require().Equal(!isSuccess, transaction.HasErrors)

	feeNumeric, ok := new(big.Int).SetString(fee, 10)
	sut.Require().True(ok)
	sut.Require().Equal(feeNumeric, transaction.Fee[0].Numeric)
	sut.Require().Equal(feeTxt, transaction.Fee[0].Text)
	sut.Require().Equal(currency, transaction.Fee[0].Currency)
	sut.Require().EqualValues(exp, transaction.Fee[0].Exp)

	sut.Require().Len(transaction.Events, len(ev))
	for i, e := range ev {
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
		sut.Require().Equal(time.Unix(), event.Completion.Unix())

		if a, ok := event.Amount["0"]; ok {
			validateAmount(sut, &a, exp, amount, amountTxt, currency)
		}

		if account, ok := event.Node["versions"]; ok {
			sut.Require().Equal(accountID, account[0].ID)
		}

		if account, ok := event.Node["sender"]; ok {
			sut.Require().Equal(senderID, account[0].ID)
		}

		if account, ok := event.Node["recipient"]; ok {
			sut.Require().Equal(recipientID, account[0].ID)

			transfers := event.Transfers[recipientID]
			sut.Require().Equal(recipientID, transfers[0].Account.ID)

			validateAmount(sut, &transfers[0].Amounts[0], exp, amount, amountTxt, currency)
		}

		// for _, a := range additional {
		// 	sut.Require().Contains(event.Additional["attributes"], a)
		// }

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
