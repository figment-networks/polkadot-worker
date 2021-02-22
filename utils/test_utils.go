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

func ValidateTransaction(sut *suite.Suite, transaction structs.Transaction, time time.Time, ev []EventValues, height uint64, isSuccess bool, blockHash, chainID, currency, epoch, trHash, fee, feeTxt string) {
	sut.Require().Equal(height, transaction.Height)
	sut.Require().Equal(blockHash, transaction.BlockHash)
	sut.Require().Equal(trHash, transaction.Hash)
	sut.Require().Equal(strconv.Itoa(int(time.Unix())), transaction.Epoch)
	sut.Require().Equal(chainID, transaction.ChainID)
	sut.Require().Equal(epoch, transaction.Epoch)
	sut.Require().Equal(time.Unix(), transaction.Time.Unix())
	sut.Require().Equal("0.0.1", transaction.Version)
	sut.Require().Equal(!isSuccess, transaction.HasErrors)

	feeNumeric, ok := new(big.Int).SetString(fee, 10)
	sut.Require().True(ok)
	sut.Require().Equal(feeNumeric, transaction.Fee[0].Numeric)
	sut.Require().Equal(feeTxt, transaction.Fee[0].Text)
	sut.Require().Equal(currency, transaction.Fee[0].Currency)
	sut.Require().Equal(10, transaction.Fee[0].Exp)

	sut.Require().Len(transaction.Events, len(ev))
	for i, e := range ev {
		sut.Require().Equal(e.Index, transaction.Events[i].ID)
		// sub.Require().Equal(kind, transaction.Events[i].Kind)
		// sub.Require().Equal(action, transaction.Events[i].Sub[0].Action)

		expectedType := e.Method
		if transaction.Events[i].Kind == "error" {
			expectedType = "error"
		}
		sut.Require().Equal([]string{expectedType}, transaction.Events[i].Sub[0].Type)
		sut.Require().Equal(e.Section, transaction.Events[i].Sub[0].Module)

	}
}
