package indexer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/figment-networks/polkadot-worker/api"
	"github.com/figment-networks/polkadot-worker/api/scale"
	"github.com/figment-networks/polkadot-worker/mapper"
	scalecodec "github.com/itering/scale.go"

	wStructs "github.com/figment-networks/polkadot-worker/structs"

	"github.com/figment-networks/indexer-manager/structs"

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
)

// PolkadotTypeSystemEvents is literally  `xxhash("System",128) + xxhash("Events",128)`
const PolkadotTypeSystemEvents = "0x26aa394eea5630e07c48ae0c9558cef780d41e5e16056765bc8461851072c9d7"

// PolkadotTypeTimeNow is literally  `xxhash("Timestamp",128) + xxhash("Now",128)`
const PolkadotTypeTimeNow = "0xf0c365c3cf59d671eb72da0e7a4113c49f1f0515f462cdcf84e0f1d6045dfcbb"

// PolkadotTypeNextFeeMultiplier is literally  `xxhash("TransactionPayment",128) + xxhash("NextFeeMultiplier",128)`
const PolkadotTypeNextFeeMultiplier = "0x3f1467a096bcd71a5b6a0c8155e208103f2edf3bdf381debe331ab7446addfdc"

// PolkadotTypeCurrentEra is literally  `xxhash("Staking",128) + xxhash("CurrentEra",128)`
const PolkadotTypeCurrentEra = "0x5f3e4907f716ac89b6347d15ececedca0b6a45321efae92aea15e0740ec7afe7"

func (c *Client) blockAndTx(ctx context.Context, logger *zap.Logger, height uint64) (block *structs.Block, transactions []*structs.Transaction, err error) {
	now := time.Now()
	ch := c.gbPool.Get()
	defer c.gbPool.Put(ch)

	if height == 0 {
		height, err = getLatestHeight(c.serverConn, c.Cache, ch)
		if err != nil {
			return nil, nil, fmt.Errorf("error getting latest height : %w", err)
		}
	}

	blH, pBlH, gpBLH, err := getBlockHashes(height, c.serverConn, c.Cache, ch)
	if err != nil {
		return nil, nil, fmt.Errorf("error unmarshaling block data: %w", err)
	}

	ddr, pblock, prv, err := getOthers(blH, pBlH, gpBLH, c.serverConn, ch)
	if err != nil {
		return nil, nil, fmt.Errorf("error unmarshaling block data: %w", err)
	}
	ddr.BlockHash = blH

	meta, err := c.getMetadata(c.serverConn, ch, gpBLH, prv.SpecName, uint(prv.SpecVersion))
	if err != nil {
		return nil, nil, fmt.Errorf("error while getting metadata: %w", err)
	}

	ddr.MetadataParent = make([]byte, len(meta.Bytes))
	copy(ddr.MetadataParent, meta.Bytes)

	txs, err := getTransactionsForHeight(c.ds, pblock, meta, int(prv.SpecVersion))
	if err != nil {
		return nil, nil, fmt.Errorf("error getTransactionsForHeight: %w", err)
	}

	log.Printf("txs %+v", txs)

	resp, err := c.proxy.DecodeData(ctx, ddr, height)
	if err != nil {
		return nil, nil, fmt.Errorf("error while decoding data: %w", err)
	}

	if block, err = mapper.BlockMapper(resp.Block, c.chainID, resp.Epoch); err != nil {
		return nil, nil, fmt.Errorf("error while mapping block: %w", err)
	}

	if len(resp.Block.Block.Extrinsics) == 0 {
		return block, nil, nil
	}

	if transactions, err = c.trMapper.TransactionsMapper(c.log, resp.Block); err != nil {
		return nil, nil, fmt.Errorf("error while mapping transactions: %w", err)
	}

	logger.Debug("Finished ", zap.Uint64("height", height), zap.Duration("from", time.Since(now)))
	return block, transactions, nil
}

func getLatestHeight(conn PolkaClient, cache *ClientCache, ch chan api.Response) (height uint64, err error) {
	conn.Send(ch, RequestFinalizedHead, "chain_getFinalizedHead", nil)
	resp := <-ch
	if resp.Error != nil {
		return 0, fmt.Errorf("response from ws is wrong: %s ", resp.Error)
	}

	conn.Send(ch, RequestTopHeader, "chain_getHeader", []interface{}{resp.Result})
	header := <-ch
	if header.Error != nil {
		return 0, fmt.Errorf("response from ws is wrong: %s ", resp.Error)
	}

	bH := &scale.BlockHeader{}
	if err := json.Unmarshal(header.Result, bH); err != nil {
		return 0, err
	}

	if bH.Number == "" {
		return 0, fmt.Errorf("response from ws is wrong: %s ", string(header.Result))
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

func getTransactionsForHeight(ds *scale.DecodeStorage, block *scale.PolkaBlock, meta *scale.MDecoder, specVer int) (transactions []scalecodec.ExtrinsicDecoder, err error) {

	for _, extrinsicRaw := range block.Contents.Extrinsics {
		eDec, err := ds.GetExtrinsic(extrinsicRaw, &meta.Decoder.Metadata, specVer)
		if err != nil {
			return transactions, err
		}
		transactions = append(transactions, eDec)
	}
	return transactions, err
}

/*
	m := scalecodec.MetadataDecoder{}
	//m.CheckRegistry()
	types.RuntimeType{}.Reg()
	c, err := ioutil.ReadFile("./polkadot.json")
	if err != nil {
		panic(err)
	}
	types.RegCustomTypes(source.LoadTypeRegistry(c))
	m.Init(utiles.HexToBytes(metadata))
	if err = m.Process(); err != nil {
		return err
	}
	option := types.ScaleDecoderOption{Metadata: &m.Metadata, Spec: 29}
	//option := types.ScaleDecoderOption{Metadata: &m.Metadata, Spec: 1055}
*/
//r := `{"call_code":"0200","call_module":"Timestamp","call_module_function":"set","era":"","extrinsic_length":10,"nonce":0,"params":[{"name":"now","type":"Compact\u003cMoment\u003e","value":1587602394}],"tip":null,"version_info":"04"}`

/*
	conn.Send(ch, RequestA, "eth_getBlockTransactionCountByNumber", []interface{}{"0x497a6d"})

	txNum := <-ch
	if txNum.Error != nil {
		return err
	}

	s := string(txNum.Result)
	if s != "" {
		//res := []byte(s[1 : len(s)-1])
		log.Println("res", height, s)
	}
	//var expected int
	//for _, v := range v {
	//	conn.Send(ch, RequestB+i, "getTransactionByBlockNumberAndIndex", []interface{}{height, 0})
	//}
	/*
		for tx := range ch { // (lukanus): has to die in it's own context
			i++
			if blockHashResp.Error != nil {
				err = blockHashResp.Error
				if i == expected {
					break
				}
				continue
			}
*/

func (c *Client) getMetadata(conn PolkaClient, ch chan api.Response, blockHash, specName string, specVer uint) (meta *scale.MDecoder, err error) {

	mDec, ok, err := c.ds.GetMDecoder(specName, specVer)
	if err != nil {
		return nil, err
	}

	if ok {
		return mDec, nil
	}

	conn.Send(ch, RequestParentMetadata, "state_getMetadata", []interface{}{blockHash})
	res := <-ch
	if res.Error != nil {
		return nil, res.Error
	}

	s := string(res.Result)
	return c.ds.SetMetadataDecoder(specVer, []byte(s[1:len(s)-1]))
}

func getBlockHashes(height uint64, conn PolkaClient, cache *ClientCache, ch chan api.Response) (blockHash, parentHash, grandparentHash string, err error) {
	var (
		expected uint8
	)

	cache.BlockHashCacheLock.RLock()
	HS, ok := cache.BlockHashCache.Get(height)
	cache.BlockHashCacheLock.RUnlock()
	if ok {
		blockHash = HS.(string)
	} else {
		conn.Send(ch, RequestBlockHash, "chain_getBlockHash", []interface{}{height})
		expected++
	}

	if height > 0 {
		cache.BlockHashCacheLock.RLock()
		pHS, ok := cache.BlockHashCache.Get(height - 1)
		cache.BlockHashCacheLock.RUnlock()

		if ok {
			parentHash = pHS.(string)
		} else {
			conn.Send(ch, RequestParentBlockHash, "chain_getBlockHash", []interface{}{height - 1})
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
			conn.Send(ch, RequestGrandparentBlockHash, "chain_getBlockHash", []interface{}{height - 2})
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
			blockHash = string(blockHashResp.Result[1 : len(blockHashResp.Result)-1])
			cache.BlockHashCacheLock.Lock()
			bh := blockHash
			cache.BlockHashCache.Add(height, bh)
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

	conn.Send(ch, RequestSystemChain, "system_chain", []interface{}{})
	conn.Send(ch, RequestBlock, "chain_getBlock", []interface{}{blockHash})

	conn.Send(ch, RequestTimestamp, "state_getStorage", []interface{}{PolkadotTypeTimeNow, blockHash})
	conn.Send(ch, RequestSystemEvents, "state_getStorage", []interface{}{PolkadotTypeSystemEvents, blockHash})

	conn.Send(ch, RequestNextFeeMultipier, "state_getStorage", []interface{}{PolkadotTypeNextFeeMultiplier, parentBlockHash})
	conn.Send(ch, RequestCurrentEra, "state_getStorage", []interface{}{PolkadotTypeCurrentEra, parentBlockHash})

	conn.Send(ch, RequestParentRuntimeVersion, "state_getRuntimeVersion", []interface{}{grandParentBlockHash})

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
