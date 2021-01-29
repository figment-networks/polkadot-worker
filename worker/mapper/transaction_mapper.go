package mapper

import (
	"strconv"
	"time"

	"github.com/figment-networks/indexer-manager/structs"
	"github.com/figment-networks/polkadothub-proxy/grpc/block/blockpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/transaction/transactionpb"

	"github.com/pkg/errors"
)

const (
	CHAIN_ID = "Polkadot"
	V_1      = "0.0.1"
)

// TransactionMap maps Block and Transaction response into database Transcation struct
func TransactionMap(blockRes *blockpb.GetByHeightResponse, transactionRes *transactionpb.GetByHeightResponse) ([]*structs.Transaction, error) {
	blockHash := blockRes.Block.BlockHash
	height := blockRes.Block.Header.Height

	var transactions []*structs.Transaction
	for _, t := range transactionRes.Transactions {
		now := time.Now()

		timeInt, err := strconv.Atoi(t.Time)
		if err != nil {
			return nil, errors.Wrap(err, "Could not parse transaction time")
		}

		transactions = append(transactions, &structs.Transaction{
			CreatedAt: &now,
			UpdatedAt: &now,
			Hash:      t.Hash,
			BlockHash: blockHash,
			Height:    uint64(height),
			Epoch:     t.Time,
			ChainID:   CHAIN_ID,
			Time:      time.Unix(int64(timeInt), 0),
			Fee:       []structs.TransactionAmount{},
			GasWanted: 0,
			GasUsed:   0,
			Memo:      "",
			Version:   V_1,
			Events:    []structs.TransactionEvent{},
			Raw:       []byte{},
			RawLog:    []byte{},
			HasErrors: !t.IsSuccess,
		})
	}

	return transactions, nil
}
