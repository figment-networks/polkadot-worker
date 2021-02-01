package mapper_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/figment-networks/polkadot-worker/worker/mapper"
	"github.com/figment-networks/polkadot-worker/worker/utils"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestBlockMapper_OK(t *testing.T) {
	chainID := "chainID"
	height := int64(120)
	hash := "0x2326841a64e0a3fff2b4bb760d316cc74b33a8a9480a28ab7e7885acba85e3cf"
	numberOfTransactions := uint64(3)
	now := time.Now()
	time := timestamppb.New(now)

	blockRes := utils.BlockResponse(height, hash, time)

	block := mapper.BlockMapper(blockRes, chainID, numberOfTransactions)

	require.Equal(t, chainID, block.ChainID)
	require.Equal(t, strconv.Itoa(int(now.Unix())), block.Epoch)
	require.Equal(t, hash, block.Hash)
	require.EqualValues(t, height, block.Height)
	require.Equal(t, numberOfTransactions, block.NumberOfTransactions)
	require.Equal(t, now.Unix(), block.Time.Unix())
}
