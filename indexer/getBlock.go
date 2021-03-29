package indexer

import (
	"context"
	"fmt"

	"github.com/figment-networks/indexer-manager/structs"
	"github.com/figment-networks/polkadot-worker/api"
	"github.com/figment-networks/polkadot-worker/mapper"
	wStructs "github.com/figment-networks/polkadot-worker/structs"

	"go.uber.org/zap"
)

const (
	RequestBlockHash = iota + 1
	RequestParentBlockHash
	RequestGrandparentBlockHash
	RequestBlock
	RequestTimestamp
	RequestSystemEvents
	RequestNextFeeMultipier
	RequestCurrentEra
	RequestParentMetadata
	RequestParentRuntimeVersion
)

// PolkadotTypeSystemEvents is literally  `xxhash("System",128) + xxhash("Events",128)`
const PolkadotTypeSystemEvents = "0x26aa394eea5630e07c48ae0c9558cef780d41e5e16056765bc8461851072c9d7"

// PolkadotTypeTimeNow is literally  `xxhash("Timestamp",128) + xxhash("Now",128)`
const PolkadotTypeTimeNow = "0xf0c365c3cf59d671eb72da0e7a4113c49f1f0515f462cdcf84e0f1d6045dfcbb"

// PolkadotTypeNextFeeMultiplier is literally  `xxhash("TransactionPayment",128) + xxhash("NextFeeMultiplier",128)`
const PolkadotTypeNextFeeMultiplier = "0x3f1467a096bcd71a5b6a0c8155e208103f2edf3bdf381debe331ab7446addfdc"

// PolkadotTypeCurrentEra is literally  `xxhash("Staking",128) + xxhash("CurrentEra",128)`
const PolkadotTypeCurrentEra = "0x5f3e4907f716ac89b6347d15ececedca0b6a45321efae92aea15e0740ec7afe7"

func blockAndTx(ctx context.Context, logger *zap.Logger, c *Client, height uint64) (block *structs.Block, transactions []*structs.Transaction, err error) {

	ch := c.gbPool.Get()
	defer c.gbPool.Put(ch)
	blH, pBlH, gpBLH, err := getBlockHashes(height, c.serverConn, c.Cache, ch)
	if err != nil {
		return nil, nil, fmt.Errorf("error unmarshaling block data: %w  ", err)
	}

	ddr, err := getOthers(blH, pBlH, gpBLH, c.serverConn, ch)
	if err != nil {
		return nil, nil, fmt.Errorf("error unmarshaling block data: %w  ", err)
	}

	ddr.BlockHash = blH

	resp, err := c.proxy.DecodeData(ctx, ddr)
	if err != nil {
		return nil, nil, err
	}

	numberOfTransactions := uint64(len(resp.Block.Block.Extrinsics))
	if block, err = mapper.BlockMapper(resp.Block, c.chainID, numberOfTransactions); err != nil {
		return nil, nil, err
	}

	if numberOfTransactions == 0 {
		return block, nil, nil
	}

	if transactions, err = c.trMapper.TransactionsMapper(c.log, resp.Block, resp.Era); err != nil {
		return nil, nil, err
	}

	return
}

func getBlockHashes(height uint64, conn *api.Conn, cache *ClientCache, ch chan api.Response) (blockHash, parentHash, grandparentHash string, err error) {
	var (
		expected uint8
	)
	conn.Requests <- api.JsonRPCSend{
		RespCH:         ch,
		JsonRPCRequest: api.JsonRPCRequest{ID: RequestBlockHash, Method: "chain_getBlockHash", Params: []interface{}{height}},
	}
	expected++

	if height > 0 {
		cache.BlockHashCacheLock.RLock()
		pHS, ok := cache.BlockHashCache.Get(height - 1)
		cache.BlockHashCacheLock.RUnlock()

		if ok {
			parentHash = pHS.(string)
		} else {
			conn.Requests <- api.JsonRPCSend{
				RespCH:         ch,
				JsonRPCRequest: api.JsonRPCRequest{ID: RequestParentBlockHash, Method: "chain_getBlockHash", Params: []interface{}{height - 1}},
			}
			expected++
		}
	}

	if height > 1 {
		cache.BlockHashCacheLock.RLock()
		gpHS, ok := cache.BlockHashCache.Get(height - 1)
		cache.BlockHashCacheLock.RUnlock()

		if ok {
			grandparentHash = gpHS.(string)
		} else {

			conn.Requests <- api.JsonRPCSend{
				RespCH:         ch,
				JsonRPCRequest: api.JsonRPCRequest{ID: RequestGrandparentBlockHash, Method: "chain_getBlockHash", Params: []interface{}{height - 2}},
			}
		}
		expected++
	}

	var i uint8
	for blockHashResp := range ch { // (lukanus): has to die in it's own context
		i++
		if blockHashResp.Error != nil {
			return blockHash, parentHash, grandparentHash, fmt.Errorf("error retrieving data from node: %w  ", err)
		}

		switch blockHashResp.ID {
		case RequestBlockHash:
			blockHash = string(blockHashResp.Result[1 : len(blockHashResp.Result)-1])
			cache.BlockHashCacheLock.Lock()
			cache.BlockHashCache.Add(height, blockHash)
			cache.BlockHashCacheLock.Unlock()
		case RequestParentBlockHash:
			parentHash = string(blockHashResp.Result[1 : len(blockHashResp.Result)-1])
		case RequestGrandparentBlockHash:
			grandparentHash = string(blockHashResp.Result[1 : len(blockHashResp.Result)-1])
		}

		if i == expected {
			break
		}

	}

	switch height {
	case 0:
		return blockHash, blockHash, blockHash, err
	case 1:
		return blockHash, parentHash, parentHash, err
	}

	return blockHash, parentHash, grandparentHash, err
}

func getOthers(blockHash, parentBlockHash, grandParentBlockHash string, conn *api.Conn, ch chan api.Response) (ddr wStructs.DecodeDataRequest, err error) {

	conn.Requests <- api.JsonRPCSend{
		RespCH:         ch,
		JsonRPCRequest: api.JsonRPCRequest{ID: RequestBlock, Method: "chain_getBlock", Params: []interface{}{blockHash}},
	}

	conn.Requests <- api.JsonRPCSend{
		RespCH:         ch,
		JsonRPCRequest: api.JsonRPCRequest{ID: RequestTimestamp, Method: "state_getStorage", Params: []interface{}{PolkadotTypeTimeNow, blockHash}},
	}

	conn.Requests <- api.JsonRPCSend{
		RespCH:         ch,
		JsonRPCRequest: api.JsonRPCRequest{ID: RequestSystemEvents, Method: "state_getStorage", Params: []interface{}{PolkadotTypeSystemEvents, blockHash}},
	}

	conn.Requests <- api.JsonRPCSend{
		RespCH:         ch,
		JsonRPCRequest: api.JsonRPCRequest{ID: RequestNextFeeMultipier, Method: "state_getStorage", Params: []interface{}{PolkadotTypeNextFeeMultiplier, parentBlockHash}},
	}

	conn.Requests <- api.JsonRPCSend{
		RespCH:         ch,
		JsonRPCRequest: api.JsonRPCRequest{ID: RequestCurrentEra, Method: "state_getStorage", Params: []interface{}{PolkadotTypeCurrentEra, parentBlockHash}},
	}

	conn.Requests <- api.JsonRPCSend{
		RespCH:         ch,
		JsonRPCRequest: api.JsonRPCRequest{ID: RequestParentMetadata, Method: "state_getMetadata", Params: []interface{}{grandParentBlockHash}},
	}

	conn.Requests <- api.JsonRPCSend{
		RespCH:         ch,
		JsonRPCRequest: api.JsonRPCRequest{ID: RequestParentRuntimeVersion, Method: "state_getRuntimeVersion", Params: []interface{}{grandParentBlockHash}},
	}

	ddr = wStructs.DecodeDataRequest{}

	var i uint8
	for res := range ch {
		if res.Error != nil {
			if i == 6 {
				err = res.Error
				break
			}
			i++
			continue
		}
		switch res.Type { // (lukanus): the []]byte(s[1 : len(s)-1])  is cutting out the quotes
		case "chain_getBlock":
			s := string(res.Result)
			ddr.Block = []byte(s[1 : len(s)-1])
		case "state_getMetadata":
			s := string(res.Result)
			ddr.MetadataParent = []byte(s[1 : len(s)-1])
		case "state_getRuntimeVersion":
			s := string(res.Result)
			ddr.RuntimeParent = []byte(s[1 : len(s)-1])
		case "state_getStorage":
			switch res.ID {
			case RequestNextFeeMultipier:
				s := string(res.Result)
				ddr.NextFeeMultipier = []byte(s[1 : len(s)-1])
			case RequestCurrentEra:
				s := string(res.Result)
				ddr.CurrentEra = []byte(s[1 : len(s)-1])
			case RequestTimestamp:
				s := string(res.Result)
				ddr.Timestamp = []byte(s[1 : len(s)-1])
			case RequestSystemEvents:
				s := string(res.Result)
				ddr.Events = []byte(s[1 : len(s)-1])
			}
		}

		if i == 6 {
			break
		}
		i++
	}

	return ddr, err
}
