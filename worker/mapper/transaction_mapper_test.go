package mapper_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/figment-networks/polkadot-worker/worker/mapper"

	"github.com/figment-networks/polkadothub-proxy/grpc/block/blockpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/event/eventpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/transaction/transactionpb"

	"github.com/stretchr/testify/require"
)

func TestTransactionMap_OK(t *testing.T) {
	chainID := "chainID"

	blockRes := &blockpb.GetByHeightResponse{
		Block: &blockpb.Block{
			BlockHash: "0x2326841a64e0a3fff2b4bb760d316cc74b33a8a9480a28ab7e7885acba85e3cf",
			Header: &blockpb.Header{
				Height: 120,
			},
		},
	}

	eventRes := &eventpb.GetByHeightResponse{
		Events: []*eventpb.Event{{
			ExtrinsicIndex: 1,
			Index:          3,
		}, {
			ExtrinsicIndex: 2,
			Index:          2,
		}},
	}

	transactionRes := &transactionpb.GetByHeightResponse{
		Transactions: []*transactionpb.Transaction{{
			ExtrinsicIndex: 1,
			PartialFee:     "1000",
			Hash:           "0xe05668b03c6e8bc1dbb61afab01d0515460693e5ade5627461c6298fcf6b8e72",
			IsSuccess:      true,
			Time:           strconv.Itoa(int(time.Now().Add(-500).Unix())),
		}},
	}

	transaction, err := mapper.TransactionMapper(chainID, blockRes, eventRes, transactionRes)

	require.Nil(t, err)

	require.Len(t, transaction, 1)

	require.Equal(t, transactionRes.Transactions[0].Hash, transaction[0].Hash)
	require.Equal(t, blockRes.Block.BlockHash, transaction[0].BlockHash)
	require.EqualValues(t, blockRes.Block.Header.Height, transaction[0].Height)
	require.Equal(t, transactionRes.Transactions[0].Time, transaction[0].Epoch)
	require.Equal(t, chainID, transaction[0].ChainID)

	timeInt, _ := strconv.Atoi(transactionRes.Transactions[0].Time)
	require.Equal(t, time.Unix(int64(timeInt), 0), transaction[0].Time)

	require.Equal(t, transactionRes.Transactions[0].PartialFee, transaction[0].Fee[0].Text)
	require.Equal(t, "0.0.1", transaction[0].Version)
	require.Equal(t, false, transaction[0].HasErrors)
}
