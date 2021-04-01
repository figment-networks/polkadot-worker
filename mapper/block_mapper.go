package mapper

import (
	"errors"

	"github.com/figment-networks/indexer-manager/structs"
	"github.com/figment-networks/indexing-engine/metrics"
	"github.com/figment-networks/polkadothub-proxy/grpc/block/blockpb"
)

// BlockMapper maps polkadothub-proxy Block to indexer-manager Block
func BlockMapper(block *blockpb.GetByHeightResponse, chainID, epoch string) (*structs.Block, error) {
	timer := metrics.NewTimer(conversionDuration.WithLabels("block"))
	defer timer.ObserveDuration()

	if block == nil {
		return nil, errors.New("Empty block response")
	}

	return &structs.Block{
		ChainID:              chainID,
		Epoch:                epoch,
		Hash:                 block.Block.BlockHash,
		Height:               uint64(block.Block.Header.Height),
		NumberOfTransactions: uint64(len(block.Block.Extrinsics)),
		Time:                 block.Block.Header.Time.AsTime(),
	}, nil
}
