package mapper_test

import (
	"testing"

	"github.com/figment-networks/polkadot-worker/mapper"
	"github.com/figment-networks/polkadot-worker/utils"

	"github.com/stretchr/testify/suite"
)

type BlockMapperTest struct {
	suite.Suite

	ChainID              string
	Epoch                string
	Height               uint64
	NumberOfTransactions uint64

	BlockResponse []utils.BlockResp
}

func (t *BlockMapperTest) SetupTest() {
	t.ChainID = "Polkadot"
	t.Epoch = "320"
	t.Height = 120
	t.NumberOfTransactions = 3

	t.BlockResponse = utils.GetBlocksResponses([2]uint64{t.Height, 4576})
}

func (t *BlockMapperTest) TestBlockMapper_OK() {
	block, err := mapper.BlockMapper(utils.BlockResponse(t.BlockResponse[0], nil, nil), t.ChainID, t.Epoch, t.NumberOfTransactions)

	t.Require().Nil(err)

	t.Require().Equal(t.ChainID, block.ChainID)
	t.Require().Equal(t.Epoch, block.Epoch)
	t.Require().Equal(t.BlockResponse[0].Hash, block.Hash)
	t.Require().EqualValues(t.Height, block.Height)
	t.Require().Equal(t.NumberOfTransactions, block.NumberOfTransactions)
	t.Require().Equal(t.BlockResponse[0].Time.Seconds, block.Time.Unix())
}

func (t *BlockMapperTest) TestBlockMapper_Error() {
	block, err := mapper.BlockMapper(nil, t.ChainID, t.Epoch, t.NumberOfTransactions)

	t.Require().Nil(block)
	t.Require().NotNil(err)
	t.Require().Contains(err.Error(), "Empty block response")
}

func TestBlockMapper(t *testing.T) {
	suite.Run(t, new(BlockMapperTest))
}
