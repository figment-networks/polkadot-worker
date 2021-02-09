package mapper

import (
	"strconv"

	"github.com/figment-networks/polkadot-worker/worker/proxy"

	"github.com/figment-networks/indexer-manager/structs"
	"github.com/figment-networks/indexing-engine/metrics"
	"github.com/figment-networks/polkadothub-proxy/grpc/block/blockpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/event/eventpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/transaction/transactionpb"
)

const (
	V_1 = "0.0.1"
)

// TransactionMapper maps Block and Transaction response into database Transcation struct
func TransactionMapper(blockRes *blockpb.GetByHeightResponse, chainID string, eventRes *eventpb.GetByHeightResponse, transactionRes *transactionpb.GetByHeightResponse) ([]*structs.Transaction, error) {
	timer := metrics.NewTimer(proxy.TransactionConversionDuration)
	defer timer.ObserveDuration()

	if blockRes == nil || eventRes == nil || transactionRes == nil {
		return nil, nil
	}

	blockHash := blockRes.Block.BlockHash
	height := blockRes.Block.Header.Height
	transactionMap := make(map[string]struct{})

	var transactions []*structs.Transaction
	for _, t := range transactionRes.Transactions {
		if _, ok := transactionMap[t.Hash]; ok {
			continue
		}

		transactionMap[t.Hash] = struct{}{}

		// timeInt, err := strconv.Atoi(t.Time)
		// if err != nil {
		// 	return nil, errors.Wrap(err, "Could not parse transaction time")
		// }

		events := []structs.TransactionEvent{}
		for _, e := range eventRes.Events {
			if e.ExtrinsicIndex == t.ExtrinsicIndex {
				events = append(events, structs.TransactionEvent{
					ID: strconv.Itoa(int(e.Index)),
				})
			}
		}

		transactions = append(transactions, &structs.Transaction{
			Hash:      t.Hash,
			BlockHash: blockHash,
			Height:    uint64(height),
			Epoch:     t.Time,
			ChainID:   chainID,
			// Time:      time.Unix(int64(timeInt), 0),
			Fee:       []structs.TransactionAmount{{Text: t.PartialFee}},
			GasWanted: 0,
			GasUsed:   0,
			Memo:      "",
			Version:   V_1,
			Events:    events,
			Raw:       []byte{},
			RawLog:    []byte{},
			HasErrors: !t.IsSuccess,
		})
	}

	return transactions, nil
}
