package mapper_test

import (
	"testing"

	"github.com/figment-networks/polkadot-worker/worker/mapper"

	"github.com/figment-networks/indexer-manager/structs"
	"github.com/figment-networks/polkadothub-proxy/grpc/block/blockpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/transaction/transactionpb"

	"github.com/stretchr/testify/require"
)

func TestTransactionMap_OK(t *testing.T) {
	blockRes := &blockpb.GetByHeightResponse{}

	transactionRes := &transactionpb.GetByHeightResponse{}

	transaction, err := mapper.TransactionMapper(blockRes, transactionRes)

	require.Nil(t, err)

	expected := structs.Transaction{}

	require.EqualValues(t, expected, &transaction)
}
