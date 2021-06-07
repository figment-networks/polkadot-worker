package mapper_test

import (
	"testing"

	"github.com/figment-networks/polkadot-worker/mapper"
	"github.com/figment-networks/polkadot-worker/utils"
	"github.com/figment-networks/polkadothub-proxy/grpc/decode/decodepb"

	"github.com/stretchr/testify/suite"
)

var blockResponse = []byte(``)

type BlockMapperTest struct {
	suite.Suite

	ChainID string
	Epoch   string

	DecodeResponse *decodepb.DecodeResponse
}

func (t *BlockMapperTest) SetupTest() {
	t.ChainID = "Polkadot"
	t.Epoch = "320"

	utils.ReadFile(t.Suite, "./../utils/decoded.json", &t.DecodeResponse)
}

func (t *BlockMapperTest) TestBlockMapper_OK() {
	block, err := mapper.BlockMapper(t.DecodeResponse.Block, t.ChainID, t.Epoch)

	t.Require().Nil(err)

	blResp := t.DecodeResponse.Block.Block
	t.Require().Equal(t.ChainID, block.ChainID)
	t.Require().Equal(t.Epoch, block.Epoch)
	t.Require().Equal(blResp.BlockHash, block.Hash)
	t.Require().Equal(uint64(blResp.Header.Height), block.Height)
	t.Require().EqualValues(len(blResp.Extrinsics), block.NumberOfTransactions)
	t.Require().Equal(blResp.Header.Time.Seconds, block.Time.Unix())
}

func (t *BlockMapperTest) TestBlockMapper_Error() {
	block, err := mapper.BlockMapper(nil, t.ChainID, t.Epoch)

	t.Require().Nil(block)
	t.Require().NotNil(err)
	t.Require().Contains(err.Error(), "Empty block response")
}

func TestBlockMapper(t *testing.T) {
	suite.Run(t, new(BlockMapperTest))
}
