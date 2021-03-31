package mapper

import (
	"errors"

	"github.com/figment-networks/indexer-manager/structs"
	"github.com/figment-networks/indexing-engine/metrics"
	"github.com/figment-networks/polkadothub-proxy/grpc/block/blockpb"
)

// BlockMapper maps polkadothub-proxy Block to indexer-manager Block
func BlockMapper(block *blockpb.GetByHeightResponse, chainID, epoch string, numberOfTransactions uint64) (*structs.Block, error) {
	timer := metrics.NewTimer(conversionDuration.WithLabels("block"))
	defer timer.ObserveDuration()

	if block == nil {
		return nil, errors.New("Empty block response")
	}

	time := block.Block.Header.Time.AsTime()

	return &structs.Block{
		ChainID:              chainID,
		Epoch:                epoch,
		Hash:                 block.Block.BlockHash,
		Height:               uint64(block.Block.Header.Height),
		NumberOfTransactions: numberOfTransactions,
		Time:                 time,
	}, nil
}
