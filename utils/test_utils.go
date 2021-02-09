package utils

import (
	"github.com/figment-networks/polkadothub-proxy/grpc/block/blockpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/event/eventpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/transaction/transactionpb"
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

func EventResponse(ev [][2]int64) *eventpb.GetByHeightResponse {
	var evts []*eventpb.Event
	for _, e := range ev {
		evts = append(evts, &eventpb.Event{
			ExtrinsicIndex: e[0],
			Index:          e[1],
		})
	}
	return &eventpb.GetByHeightResponse{
		Events: evts,
	}
}

func TransactionResponse(index int64, isSuccess bool, fee, hash, time string) *transactionpb.GetByHeightResponse {
	return &transactionpb.GetByHeightResponse{
		Transactions: []*transactionpb.Transaction{{
			ExtrinsicIndex: index,
			PartialFee:     fee,
			Hash:           hash,
			IsSuccess:      isSuccess,
			Time:           time,
		}},
	}
}
