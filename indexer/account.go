package indexer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/figment-networks/polkadot-worker/api"
	"github.com/figment-networks/polkadot-worker/api/scale"
	"github.com/itering/scale.go/types"
	"github.com/itering/scale.go/utiles"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

var ErrTimeout = errors.New("timeout after 30 seconds")

func (c *Client) GetAccount(ctx context.Context, logger *zap.Logger, height uint64, accountID string) (pai scale.PolkaAccountInfo, err error) {
	now := time.Now()
	ch := c.gbPool.Get()

	if height == 0 {
		height, err = getLatestHeight(c.serverConn, c.Cache, ch)
		if err != nil {
			c.gbPool.Put(ch)
			return pai, fmt.Errorf("error getting latest height : %w", err)
		}
	}

	blH, _, _, err := GetBlockHashes(height, c.serverConn, c.Cache, ch)
	if err != nil {
		c.gbPool.Put(ch)
		return pai, fmt.Errorf("error unmarshaling block data: %w", err)
	}

	prm := &scale.PolkaRuntimeVersion{}
	err = c.serverConn.Send(ch, RequestParentRuntimeVersion, "state_getRuntimeVersion", []interface{}{blH})
	if err != nil {
		c.gbPool.Put(ch) // (lukanus): only because we really know that Send won't pass this channel anywhere.
		return pai, err
	}

RuntimeVersionLoop:
	for {
		select {
		case resp := <-ch:
			if resp.Error != nil {
				c.gbPool.Put(ch)
				return pai, fmt.Errorf("response from ws is wrong: %s ", resp.Error)
			}
			if resp.Type != "state_getRuntimeVersion" {
				c.gbPool.Put(ch)
				return pai, errors.New("wrong data returned")
			}
			if len(resp.Result) == 0 {
				c.gbPool.Put(ch)
				return pai, fmt.Errorf("response from ws is empty")
			}
			if err = json.Unmarshal(resp.Result, prm); err != nil {
				c.gbPool.Put(ch)
				return pai, fmt.Errorf("error unmarshaling state_getRuntimeVersion: %w", err)
			}
			break RuntimeVersionLoop
		case <-time.After(time.Second * 30):
			go cleanupTimeoutedCh(logger, ch, 1)
			return pai, ErrTimeout
		}
	}

	meta, err := c.GetMetadata(c.serverConn, ch, blH, prm.SpecName, uint(prm.SpecVersion))
	if err != nil {
		c.gbPool.Put(ch)
		return pai, fmt.Errorf("error while getting metadata: %w", err)
	}

	pai, err = getAccountData(logger, c.serverConn, ch, meta, int(prm.SpecVersion), blH, accountID)
	logger.Debug("Finished ", zap.String("account", accountID), zap.Uint64("height", height), zap.Duration("from", time.Since(now)))
	if !errors.Is(err, ErrTimeout) { // timeout
		c.gbPool.Put(ch)
	}

	return pai, err
}

func cleanupTimeoutedCh(logger *zap.Logger, ch chan api.Response, num uint) {
	defer func() {
		if r := recover(); r != nil {
			logger.Info("Recovered panic from timeout")
			logger.Sync()
		}
	}()

	var i uint
CLEANUP_LOOP:
	for {
		select {
		case <-time.After(10 * time.Minute):
			break CLEANUP_LOOP
		case <-ch:
			i++
			if i == num {
				break CLEANUP_LOOP
			}
		}
	}

	close(ch)
}

func getAccountData(logger *zap.Logger, conn PolkaClient, ch chan api.Response, meta *scale.MDecoder, specVer int, blockHash string, accountID string) (pai scale.PolkaAccountInfo, err error) {

	aReq, err := scale.AccountRequest(accountID)
	if err != nil {
		return pai, nil
	}

	err = conn.Send(ch, RequestSystemAccount, "state_getStorage", []interface{}{aReq, blockHash})
	if err != nil {
		return pai, err
	}

	var respData string
RuntimeVersionLoop:
	for {
		select {
		case resp := <-ch:
			if resp.Error != nil {
				return pai, fmt.Errorf("response from ws is wrong: %s ", resp.Error)
			}
			if resp.Type != "state_getStorage" {
				return pai, errors.New("wrong data returned")
			}
			if len(resp.Result) == 0 {
				return pai, fmt.Errorf("response from ws is empty")
			}

			respData = string(resp.Result[1 : len(resp.Result)-1])
			break RuntimeVersionLoop
		case <-time.After(time.Second * 30):
			go cleanupTimeoutedCh(logger, ch, 1)
			return pai, ErrTimeout
		}
	}

	sd := &types.ScaleDecoder{}
	sd.Init(types.ScaleBytes{Data: utiles.HexToBytes(respData)}, &types.ScaleDecoderOption{
		Metadata: &meta.Decoder.Metadata,
		Spec:     specVer,
	})
	as := sd.ProcessAndUpdateData("AccountInfo")

	pai = scale.PolkaAccountInfo{}
	inf := as.(map[string]interface{})

	var ok bool
	nonce := inf["nonce"]
	pai.Nonce, ok = nonce.(uint32)
	if !ok {
		return pai, errors.New("error decoding nonce")
	}
	consumers := inf["consumers"]
	pai.Consumers, ok = consumers.(uint32)
	if !ok {
		return pai, errors.New("error decoding consumers")
	}
	providers := inf["providers"]
	pai.Providers, ok = providers.(uint32)
	if !ok {
		return pai, errors.New("error decoding providers")
	}

	data2 := inf["data"]
	inf2, ok := data2.(map[string]interface{})
	if !ok {
		return pai, errors.New("error decoding data")
	}

	free := inf2["free"]
	f2, ok := free.(decimal.Decimal)
	if !ok {
		return pai, errors.New("error decoding free")
	}
	pai.Data.Free = f2.String()

	reserved := inf2["reserved"]
	r2, ok := reserved.(decimal.Decimal)
	if !ok {
		return pai, errors.New("error decoding reserved")
	}
	pai.Data.Reserved = r2.String()

	miscFrozen := inf2["miscFrozen"]
	mf2, ok := miscFrozen.(decimal.Decimal)
	if !ok {
		return pai, errors.New("error decoding misc frozen")
	}
	pai.Data.MiscFrozen = mf2.String()

	feeFrozen := inf2["feeFrozen"]
	ff2, ok := feeFrozen.(decimal.Decimal)
	if !ok {
		return pai, errors.New("error decoding fee frozen")
	}
	pai.Data.FeeFrozen = ff2.String()
	return pai, nil

}
