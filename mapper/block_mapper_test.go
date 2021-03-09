package mapper_test

import (
	"testing"

	"github.com/figment-networks/polkadot-worker/mapper"
	"github.com/figment-networks/polkadot-worker/proxy"
	"github.com/figment-networks/polkadot-worker/utils"

	"github.com/figment-networks/indexing-engine/metrics"

	"github.com/stretchr/testify/require"
)

func TestBlockMapper_OK(t *testing.T) {
	var height uint64 = 120
	var numberOfTransactions uint64 = 3
	var chainID string = "Polkadot"

	conversionDuration := metrics.MustNewHistogramWithTags(metrics.HistogramOptions{})
	proxy.BlockConversionDuration = conversionDuration.WithLabels("block")

	blockRes := utils.GetBlocksResponses([2]uint64{height, 4576})

	block := mapper.BlockMapper(utils.BlockResponse(blockRes[0]), chainID, numberOfTransactions)

	require.Equal(t, chainID, block.ChainID)
	require.Equal(t, blockRes[0].Hash, block.Hash)
	require.EqualValues(t, height, block.Height)
	require.Equal(t, numberOfTransactions, block.NumberOfTransactions)
	require.Equal(t, blockRes[0].Time.Seconds, block.Time.Unix())
}
