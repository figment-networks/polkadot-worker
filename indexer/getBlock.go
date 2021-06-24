package indexer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/figment-networks/polkadot-worker/api"
	"github.com/figment-networks/polkadot-worker/api/scale"
	"github.com/figment-networks/polkadot-worker/mapper"
	wStructs "github.com/figment-networks/polkadot-worker/structs"

	"github.com/figment-networks/indexer-search/structs"

	"go.uber.org/zap"
)

var (
	mutex = sync.Mutex{}
)

const (
	RequestBlockHash = iota + 1
	RequestSystemChain
	RequestFinalizedHead
	RequestTopHeader
	RequestParentBlockHash
	RequestGrandparentBlockHash
	RequestBlock
	RequestTimestamp
	RequestSystemEvents
	RequestNextFeeMultipier
	RequestCurrentEra
	RequestParentMetadata
	RequestParentRuntimeVersion
	RequestSystemAccount
)

// PolkadotTypeSystemEvents is literally  `xxhash("System",128) + xxhash("Events",128)`
const PolkadotTypeSystemEvents = "0x26aa394eea5630e07c48ae0c9558cef780d41e5e16056765bc8461851072c9d7"

// PolkadotTypeTimeNow is literally  `xxhash("Timestamp",128) + xxhash("Now",128)`
const PolkadotTypeTimeNow = "0xf0c365c3cf59d671eb72da0e7a4113c49f1f0515f462cdcf84e0f1d6045dfcbb"

// PolkadotTypeNextFeeMultiplier is literally  `xxhash("TransactionPayment",128) + xxhash("NextFeeMultiplier",128)`
const PolkadotTypeNextFeeMultiplier = "0x3f1467a096bcd71a5b6a0c8155e208103f2edf3bdf381debe331ab7446addfdc"

// PolkadotTypeCurrentEra is literally  `xxhash("Staking",128) + xxhash("CurrentEra",128)`
const PolkadotTypeCurrentEra = "0x5f3e4907f716ac89b6347d15ececedca0b6a45321efae92aea15e0740ec7afe7"

var ddrCount = 0

func (c *Client) blockAndTx(ctx context.Context, height uint64) (block structs.BlockWithMeta, transactions []structs.TransactionWithMeta, err error) {
	now := time.Now()
	ch := c.gbPool.Get()
	defer c.gbPool.Put(ch)

	if height == 0 {
		height, err = getLatestHeight(c.serverConn, c.Cache, ch)
		if err != nil {
			return structs.BlockWithMeta{}, nil, fmt.Errorf("error getting latest height : %w", err)
		}
	}

	blH, pBlH, gpBLH, err := GetBlockHashes(height, c.serverConn, c.Cache, ch)
	if err != nil {
		return structs.BlockWithMeta{}, nil, fmt.Errorf("error unmarshaling block data: %w", err)
	}

	ddr, pblock, prv, err := getOthers(blH, pBlH, gpBLH, c.serverConn, ch)
	if err != nil {
		return structs.BlockWithMeta{}, nil, fmt.Errorf("error unmarshaling block data: %w", err)
	}
	ddr.BlockHash = blH

	meta, err := c.GetMetadata(c.serverConn, ch, gpBLH, prv.SpecName, uint(prv.SpecVersion))
	if err != nil {
		return structs.BlockWithMeta{}, nil, fmt.Errorf("error while getting metadata: %w", err)
	}

	ddr.MetadataParent = make([]byte, len(meta.Bytes))
	copy(ddr.MetadataParent, meta.Bytes)

	txs, err := GetTransactionsForHeight(c.ds, pblock, meta, int(prv.SpecVersion))
	if err != nil {
		return structs.BlockWithMeta{}, nil, fmt.Errorf("error getTransactionsForHeight: %w", err)
	}

	resp, err := c.proxy.DecodeData(ctx, ddr, height)
	if err != nil {
		return structs.BlockWithMeta{}, nil, fmt.Errorf("error while decoding data: %w", err)
	}

	b, err := mapper.BlockMapper(resp.Block, c.chainID, resp.Epoch)
	if err != nil {
		return structs.BlockWithMeta{}, nil, fmt.Errorf("error while mapping block: %w", err)
	}

	block = structs.BlockWithMeta{
		Network: c.network,
		ChainID: c.chainID,
		Version: "0.0.1",
		Block:   *b,
	}

	if len(resp.Block.Block.Extrinsics) == 0 {
		return block, nil, nil
	}

	trs, err := c.trMapper.TransactionsMapper(resp.Block)
	if err != nil {
		return structs.BlockWithMeta{}, nil, fmt.Errorf("error while mapping transactions: %w", err)
	}

	transactions = make([]structs.TransactionWithMeta, len(trs))

	// pair transactions with ids
	var found bool
	for k, t := range trs {
		found = false
	TXS_LOOP:
		for extIndex, rawTx := range txs {
			if t.Hash[2:] == rawTx.ExtrinsicHash {
				found = true
				for in, ev := range t.Events {
					ev.ID = fmt.Sprintf("%d-%d", b.Height, extIndex)
					t.Events[in] = ev
				}
				transactions[k] = structs.TransactionWithMeta{
					Network:     c.network,
					ChainID:     c.chainID,
					Version:     "0.0.1",
					Transaction: *t,
				}
				break TXS_LOOP
			}
		}
		if !found {
			if len(txs) >= k+1 {
				candidateTx := txs[k]
				for in, ev := range t.Events {
					if strings.ToLower(candidateTx.CallModule.Name) == ev.Module && ev.Kind == "Extrinsic" {
						ev.ID = fmt.Sprintf("%d-%d", b.Height, k)
						t.Events[in] = ev
					}
				}
				transactions[k] = structs.TransactionWithMeta{
					Network:     c.network,
					ChainID:     c.chainID,
					Version:     "0.0.1",
					Transaction: *t,
				}
			}
		}
	}

	c.log.Debug("Finished ", zap.Uint64("height", height), zap.Duration("from", time.Since(now)))
	return block, transactions, nil
}

func getLatestHeight(conn PolkaClient, cache *ClientCache, ch chan api.Response) (height uint64, err error) {
	err = conn.Send(ch, RequestFinalizedHead, "chain_getFinalizedHead", nil)
	if err != nil {
		return 0, err
	}
	resp := <-ch
	if resp.Error != nil {
		return 0, fmt.Errorf("response from ws is wrong (chain_getFinalizedHead): %s ", resp.Error)
	}

	err = conn.Send(ch, RequestTopHeader, "chain_getHeader", []interface{}{resp.Result})
	if err != nil {
		return 0, err
	}
	header := <-ch
	if header.Error != nil {
		return 0, fmt.Errorf("response from ws is wrong (chain_getHeader): %s ", resp.Error)
	}

	bH := &scale.BlockHeader{}
	if err := json.Unmarshal(header.Result, bH); err != nil {
		return 0, err
	}

	if bH.Number == "" {
		return 0, fmt.Errorf("response from ws is wrong (Number is empty): %s ", string(header.Result))
	}

	numberStr := strings.Replace(bH.Number, "0x", "", -1)
	n := new(big.Int)
	n.SetString(numberStr, 16)
	height = n.Uint64()

	if err == nil {
		blockHash := string(resp.Result[1 : len(resp.Result)-1])
		cache.BlockHashCacheLock.Lock()
		cache.BlockHashCache.Add(height, blockHash)
		cache.BlockHashCacheLock.Unlock()
	}
	return height, err
}

func GetTransactionsForHeight(ds *scale.DecodeStorage, block *scale.PolkaBlock, meta *scale.MDecoder, specVer int) (transactions []scale.ScaleExtrinsic, err error) {
	for _, extrinsicRaw := range block.Contents.Extrinsics {
		eDec, err := ds.GetExtrinsic(extrinsicRaw, &meta.Decoder.Metadata, specVer)
		if err != nil {
			return transactions, err
		}
		transactions = append(transactions, eDec)
	}
	return transactions, err
}

func (c *Client) GetMetadata(conn PolkaClient, ch chan api.Response, blockHash, specName string, specVer uint) (meta *scale.MDecoder, err error) {

	mDec, ok, err := c.ds.GetMDecoder(specName, specVer)
	if err != nil {
		return nil, err
	}

	if ok {
		return mDec, nil
	}

	err = conn.Send(ch, RequestParentMetadata, "state_getMetadata", []interface{}{blockHash})
	if err != nil {
		return nil, err
	}
	res := <-ch
	if res.Error != nil {
		return nil, res.Error
	}

	s := string(res.Result)
	return c.ds.SetMetadataDecoder(specVer, []byte(s[1:len(s)-1]))
}

func GetBlockHashes(height uint64, conn PolkaClient, cache *ClientCache, ch chan api.Response) (blockHash, parentHash, grandparentHash string, err error) {
	var (
		expected uint8
	)

	cache.BlockHashCacheLock.RLock()
	HS, ok := cache.BlockHashCache.Get(height)
	cache.BlockHashCacheLock.RUnlock()
	if ok {
		blockHash = HS.(string)
	} else {
		err = conn.Send(ch, RequestBlockHash, "chain_getBlockHash", []interface{}{height})
		if err != nil {
			return "", "", "", err
		}
		expected++
	}

	if height > 0 {
		cache.BlockHashCacheLock.RLock()
		pHS, ok := cache.BlockHashCache.Get(height - 1)
		cache.BlockHashCacheLock.RUnlock()

		if ok {
			parentHash = pHS.(string)
		} else {
			err = conn.Send(ch, RequestParentBlockHash, "chain_getBlockHash", []interface{}{height - 1})
			if err != nil {
				return "", "", "", err
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
			err = conn.Send(ch, RequestGrandparentBlockHash, "chain_getBlockHash", []interface{}{height - 2})
			if err != nil {
				return "", "", "", err
			}
			expected++
		}
	}

	if expected == 0 {
		return blockHash, parentHash, grandparentHash, nil
	}

	var i uint8
	for blockHashResp := range ch { // (lukanus): has to die in it's own context
		i++
		if blockHashResp.Error != nil {
			err = blockHashResp.Error
			if i == expected {
				break
			}
			continue
		}

		switch blockHashResp.ID {
		case RequestBlockHash:
			if len(blockHashResp.Result) == 0 {
				err = errors.New("block hash is empty ")
			} else {
				blockHash = string(blockHashResp.Result[1 : len(blockHashResp.Result)-1])
				cache.BlockHashCacheLock.Lock()
				bh := blockHash
				cache.BlockHashCache.Add(height, bh)
				cache.BlockHashCacheLock.Unlock()
			}
		case RequestParentBlockHash:
			if len(blockHashResp.Result) == 0 {
				err = errors.New("parent hash is empty ")
			} else {
				parentHash = string(blockHashResp.Result[1 : len(blockHashResp.Result)-1])
			}
		case RequestGrandparentBlockHash:
			if len(blockHashResp.Result) == 0 {
				err = errors.New("grandparent hash is empty ")
			} else {
				grandparentHash = string(blockHashResp.Result[1 : len(blockHashResp.Result)-1])
			}
		}

		if i == expected {
			break
		}
	}

	if err != nil {
		return blockHash, parentHash, grandparentHash, fmt.Errorf("error retrieving data from node: %w  ", err)
	}

	switch height {
	case 0:
		return blockHash, blockHash, blockHash, err
	case 1:
		return blockHash, parentHash, parentHash, err
	}

	return blockHash, parentHash, grandparentHash, err
}

func getOthers(blockHash, parentBlockHash, grandParentBlockHash string, conn PolkaClient, ch chan api.Response) (ddr wStructs.DecodeDataRequest, block *scale.PolkaBlock, prm *scale.PolkaRuntimeVersion, err error) {

	if err = conn.Send(ch, RequestSystemChain, "system_chain", []interface{}{}); err != nil {
		return ddr, block, prm, err
	}

	if err = conn.Send(ch, RequestBlock, "chain_getBlock", []interface{}{blockHash}); err != nil {
		return ddr, block, prm, err
	}

	if err = conn.Send(ch, RequestTimestamp, "state_getStorage", []interface{}{PolkadotTypeTimeNow, blockHash}); err != nil {
		return ddr, block, prm, err
	}

	if err = conn.Send(ch, RequestSystemEvents, "state_getStorage", []interface{}{PolkadotTypeSystemEvents, blockHash}); err != nil {
		return ddr, block, prm, err
	}

	if err = conn.Send(ch, RequestNextFeeMultipier, "state_getStorage", []interface{}{PolkadotTypeNextFeeMultiplier, parentBlockHash}); err != nil {
		return ddr, block, prm, err
	}

	if err = conn.Send(ch, RequestCurrentEra, "state_getStorage", []interface{}{PolkadotTypeCurrentEra, parentBlockHash}); err != nil {
		return ddr, block, prm, err
	}

	if err = conn.Send(ch, RequestParentRuntimeVersion, "state_getRuntimeVersion", []interface{}{grandParentBlockHash}); err != nil {
		return ddr, block, prm, err
	}

	ddr = wStructs.DecodeDataRequest{}
	block = &scale.PolkaBlock{}
	prm = &scale.PolkaRuntimeVersion{}
	var i uint8
	for res := range ch {
		if res.Error != nil {
			if i == 7 {
				err = res.Error
				break
			}
			i++
			continue
		}
		switch res.Type { // (lukanus): the []]byte(s[1 : len(s)-1])  is cutting out the quotes
		case "system_chain":
			s := string(res.Result)
			ddr.Chain = s[1 : len(s)-1]
		case "chain_getBlock":
			s := string(res.Result)
			ddr.Block = []byte(s[1 : len(s)-1])
			err = json.Unmarshal(res.Result, block)
		case "state_getRuntimeVersion":
			s := string(res.Result)
			ddr.RuntimeParent = []byte(s[1 : len(s)-1])
			err = json.Unmarshal(res.Result, prm)
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

	return ddr, block, prm, err
}
