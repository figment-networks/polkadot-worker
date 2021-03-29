package indexer

import (
	"context"
	"fmt"
	"log"

	"github.com/figment-networks/indexer-manager/structs"
	"github.com/figment-networks/polkadot-worker/api"
	"github.com/figment-networks/polkadot-worker/mapper"
	wStructs "github.com/figment-networks/polkadot-worker/structs"

	"go.uber.org/zap"
)

const (
	RequestBlockHash = iota + 1
	RequestParentBlockHash
	RequestBlock
	RequestSystemEvents
	RequestParentMetadata
	RequestParentRuntimeVersion
)

// systemEvents is literally  `xxhash("System",128) + xxhash("Events",128)`
const systemEvents = "0x26aa394eea5630e07c48ae0c9558cef780d41e5e16056765bc8461851072c9d7"

type PolkaBlock struct {
	hash string `json:"hash"`
}

func blockAndTx(ctx context.Context, logger *zap.Logger, c *Client, height uint64) (block *structs.Block, transactions []*structs.Transaction, err error) {

	ch := make(chan api.Response, 10) // TODO(lukanus):make pool
	defer close(ch)
	blH, pBlH, err := getBlockHashes(height, c.serverConn, ch)
	if err != nil {
		return nil, nil, fmt.Errorf("error unmarshaling block data: %w  ", err)
	}

	ddr, err := getOthers(blH, pBlH, c.serverConn, ch)
	if err != nil {
		return nil, nil, fmt.Errorf("error unmarshaling block data: %w  ", err)
	}

	ddr.BlockHash = blH

	resp, err := c.proxy.DecodeData(ctx, ddr)
	if err != nil {
		return
	}

	numberOfTransactions := uint64(len(resp.Block.Block.Extrinsics))
	if block, err = mapper.BlockMapper(resp.Block, c.chainID, numberOfTransactions); err != nil {
		return nil, nil, err
	}

	if numberOfTransactions == 0 {
		return block, nil, nil
	}

	if transactions, err = c.trMapper.TransactionsMapper(c.log, resp.Block, nil); err != nil {
		return nil, nil, err
	}

	return
}

func getBlockHashes(height uint64, conn *api.Conn, ch chan api.Response) (blockHash, parentHash string, err error) {

	conn.Requests <- api.JsonRPCSend{
		RespCH:         ch,
		JsonRPCRequest: api.JsonRPCRequest{ID: RequestBlockHash, Method: "chain_getBlockHash", Params: []interface{}{height}},
	}

	conn.Requests <- api.JsonRPCSend{
		RespCH:         ch,
		JsonRPCRequest: api.JsonRPCRequest{ID: RequestParentBlockHash, Method: "chain_getBlockHash", Params: []interface{}{height - 1}},
	}

	var i uint8
	for blockHashResp := range ch { // (lukanus): has to die in it's own context
		if blockHashResp.Error != nil {
			return blockHash, parentHash, fmt.Errorf("error retrieving data from node: %w  ", err)
		}

		switch blockHashResp.ID {
		case RequestBlockHash:
			blockHash = string(blockHashResp.Result[1 : len(blockHashResp.Result)-1])
		case RequestParentBlockHash:
			parentHash = string(blockHashResp.Result[1 : len(blockHashResp.Result)-1])
		}

		if i == 1 {
			break
		}
		i++

	}

	return blockHash, parentHash, err

}

func getOthers(blockHash, parentBlockHash string, conn *api.Conn, ch chan api.Response) (ddr wStructs.DecodeDataRequest, err error) {

	conn.Requests <- api.JsonRPCSend{
		RespCH:         ch,
		JsonRPCRequest: api.JsonRPCRequest{ID: RequestBlock, Method: "chain_getBlock", Params: []interface{}{blockHash}},
	}
	/*
		blockResp := <-ch
		pBlck := &PolkaBlock{}
		if err = json.Unmarshal(blockResp.Result, pBlck); err != nil {
			return ddr, fmt.Errorf("error unmarshaling block data: %w  ", err)
		}

		ddr.Block = blockResp.Result
	*/
	//	log.Printf("blockResp.Result %+v", blockResp.Result)
	conn.Requests <- api.JsonRPCSend{
		RespCH:         ch,
		JsonRPCRequest: api.JsonRPCRequest{ID: RequestParentMetadata, Method: "state_getMetadata", Params: []interface{}{parentBlockHash}},
	}

	conn.Requests <- api.JsonRPCSend{
		RespCH:         ch,
		JsonRPCRequest: api.JsonRPCRequest{ID: RequestSystemEvents, Method: "state_getStorage", Params: []interface{}{systemEvents, parentBlockHash}},
	}

	conn.Requests <- api.JsonRPCSend{
		RespCH:         ch,
		JsonRPCRequest: api.JsonRPCRequest{ID: RequestParentRuntimeVersion, Method: "state_getRuntimeVersion", Params: []interface{}{parentBlockHash}},
	}

	ddr = wStructs.DecodeDataRequest{}

	var i uint8
	for res := range ch {
		if res.Error != nil {
			log.Println("errr", res.Error)
			if i == 3 {
				break
			}
			i++
			continue
		}

		switch res.Type {
		case "chain_getBlock":
			s := string(res.Result)
			ddr.Block = []byte(s[1 : len(s)-1])
		case "state_getMetadata":
			s := string(res.Result)
			ddr.MetadataParent = []byte(s[1 : len(s)-1]) // (lukanus): cut out the quotes
		case "state_getRuntimeVersion":
			s := string(res.Result)
			ddr.RuntimeParent = []byte(s[1 : len(s)-1]) // (lukanus): cut out the quotes
		case "state_getStorage":
			switch res.ID {
			case RequestSystemEvents:
				s := string(res.Result)
				ddr.Events = []byte(s[1 : len(s)-1]) // (lukanus): cut out the quotes
			}
		}

		if i == 3 {
			break
		}
		i++
	}

	return ddr, nil
}
