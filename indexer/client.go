package indexer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/figment-networks/polkadot-worker/api"
	"github.com/figment-networks/polkadot-worker/api/scale"
	"github.com/figment-networks/polkadot-worker/mapper"
	wStructs "github.com/figment-networks/polkadot-worker/structs"

	rStructs "github.com/figment-networks/indexer-manager/structs"
	cStructs "github.com/figment-networks/indexer-manager/worker/connectivity/structs"
	"github.com/figment-networks/indexing-engine/metrics"
	"github.com/figment-networks/indexing-engine/structs"
	"github.com/figment-networks/indexing-engine/worker/process/ranged"
	searchHTTP "github.com/figment-networks/indexing-engine/worker/store/transport/http"
	"github.com/figment-networks/polkadothub-proxy/grpc/account/accountpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/block/blockpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/chain/chainpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/decode/decodepb"
	"github.com/figment-networks/polkadothub-proxy/grpc/event/eventpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/transaction/transactionpb"
	lru "github.com/hashicorp/golang-lru"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ClientIface interface
type ClientIface interface {
	GetAccountBalance(ctx context.Context, account string, height uint64) (*accountpb.GetByHeightResponse, error)
	GetBlockByHeight(ctx context.Context, height uint64) (*blockpb.GetByHeightResponse, error)
	GetMetaByHeight(ctx context.Context, height uint64) (*chainpb.GetMetaByHeightResponse, error)
	GetHead(ctx context.Context) (*chainpb.GetHeadResponse, error)
	GetEventsByHeight(ctx context.Context, height uint64) (*eventpb.GetByHeightResponse, error)
	GetTransactionsByHeight(ctx context.Context, height uint64) (*transactionpb.GetByHeightResponse, error)

	DecodeData(ctx context.Context, ddr wStructs.DecodeDataRequest, height uint64) (*decodepb.DecodeResponse, error)
}

type PolkaClient interface {
	Send(resp chan api.Response, id uint64, method string, params []interface{}) error
}

const page = 100

var (
	// ErrBadRequest is returned when cannot unmarshal message
	ErrBadRequest = errors.New("bad request")

	getAccountBalanceDuration *metrics.GroupObserver
	getTransactionDuration    *metrics.GroupObserver
	getLatestDuration         *metrics.GroupObserver
)

type ClientCache struct {
	BlockHashCache     *lru.Cache
	BlockHashCacheLock sync.RWMutex
}

// Client connecting to indexer-manager
type Client struct {
	Cache *ClientCache

	chainID         string
	currency        string
	exp             int
	maxHeightsToGet uint64
	network         string

	serverConn PolkaClient
	gbPool     *getBlockPool

	log     *zap.Logger
	proxy   ClientIface
	sLock   sync.Mutex
	streams map[uuid.UUID]*cStructs.StreamAccess

	ds          *scale.DecodeStorage
	Reqester    *ranged.RangeRequester
	searchStore *searchHTTP.HTTPStore

	abMapper *mapper.AccountBalanceMapper
	trMapper *mapper.TransactionMapper
}

// NewClient is a indexer-manager Client constructor
func NewClient(log *zap.Logger, proxy ClientIface, ds *scale.DecodeStorage, ss *searchHTTP.HTTPStore, serverConn PolkaClient, exp int, maxHeightsToGet uint64, chainID, currency, network string) *Client {
	getAccountBalanceDuration = endpointDuration.WithLabels("getAccountBalance")
	getTransactionDuration = endpointDuration.WithLabels("getTransactions")
	getLatestDuration = endpointDuration.WithLabels("getLatest")

	newLru, err := lru.New(3000)
	if err != nil {
		panic(fmt.Errorf("cache cannot be defined: %w", err)) // we really need to fatal here. this should not happen.
	}

	ic := &Client{
		chainID:         chainID,
		currency:        currency,
		exp:             exp,
		maxHeightsToGet: maxHeightsToGet,
		network:         network,
		gbPool:          NewGetBlockPool(300),
		Cache: &ClientCache{
			BlockHashCache: newLru,
		},
		ds:          ds,
		searchStore: ss,
		log:         log,
		proxy:       proxy,
		serverConn:  serverConn,
		streams:     make(map[uuid.UUID]*cStructs.StreamAccess),
		abMapper:    mapper.NewAccountBalanceMapper(exp, currency),
		trMapper:    mapper.NewTransactionMapper(exp, log, chainID, currency),
	}

	ic.Reqester = ranged.NewRangeRequester(ic, 20)

	return ic
}

// RegisterStream adds new listeners to the stream
func (c *Client) RegisterStream(ctx context.Context, stream *cStructs.StreamAccess) error {
	c.log.Debug("Register indexer-manager client stream", zap.Stringer("streamID", stream.StreamID))

	c.sLock.Lock()
	defer c.sLock.Unlock()
	c.streams[stream.StreamID] = stream

	for i := 0; i < 40; i++ {
		go c.Run(ctx, stream)
	}

	return nil
}

// CloseStream closes connection with indexer-manager
func (c *Client) CloseStream(ctx context.Context, streamID uuid.UUID) error {
	c.sLock.Lock()
	defer c.sLock.Unlock()

	c.log.Debug("Close indexer-manager client stream", zap.Stringer("streamID", streamID))
	delete(c.streams, streamID)

	return nil
}

// Run listens stream events
func (c *Client) Run(ctx context.Context, stream *cStructs.StreamAccess) {
	for {
		select {
		case <-ctx.Done():
			c.sLock.Lock()
			delete(c.streams, stream.StreamID)
			c.sLock.Unlock()
			return
		case <-stream.Finish:
			return
		case taskRequest := <-stream.RequestListener:
			c.log.Debug("Received task request", zap.Stringer("taskID", taskRequest.Id), zap.String("type", taskRequest.Type))

			ctxWithTimeout, cancel := context.WithTimeout(ctx, 40*time.Minute)
			defer cancel()

			switch taskRequest.Type {
			case rStructs.ReqIDAccountBalance:
				c.GetAccountBalance(ctxWithTimeout, taskRequest, stream)
			case rStructs.ReqIDGetTransactions:
				c.GetTransactions(ctxWithTimeout, taskRequest, stream)
			case rStructs.ReqIDGetLatestMark:
				c.GetLatestMark(ctxWithTimeout, taskRequest, stream)
			case rStructs.ReqIDLatestData:
				c.GetLatestData(ctxWithTimeout, taskRequest, stream)
			default:
				stream.Send(cStructs.TaskResponse{
					Id: taskRequest.Id,
					Error: cStructs.TaskError{
						Msg: fmt.Sprintf("Unknown request %s", taskRequest.Type),
					},
					Final: true,
				})
			}
		}
	}
}

// GetAccountBalance returns account balance
func (c *Client) GetAccountBalance(ctx context.Context, tr cStructs.TaskRequest, stream *cStructs.StreamAccess) {
	timer := metrics.NewTimer(getAccountBalanceDuration)
	defer timer.ObserveDuration()

	var hr structs.HeightAccount
	var err error

	if err = json.Unmarshal(tr.Payload, &hr); hr.Account == "" || hr.Height == 0 {
		err = ErrBadRequest
	}

	if err != nil {
		c.log.Debug("Cannot unmarshal payload", zap.String("contents", string(tr.Payload)))
		stream.Send(cStructs.TaskResponse{
			Id: tr.Id,
			Error: cStructs.TaskError{
				Msg: fmt.Sprintf("Cannot unmarshal payload: %s", err.Error()),
			},
			Final: true,
		})
		return
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	balanceSummary, err := c.getBalanceSummary(ctx, stream, hr.Account, hr.Height)
	if err != nil {
		stream.Send(cStructs.TaskResponse{
			Id: tr.Id,
			Error: cStructs.TaskError{
				Msg: fmt.Sprintf("Could not send Account Balance: %s", err.Error()),
			},
			Final: true,
		})
		return
	}

	tResp := cStructs.TaskResponse{Id: tr.Id, Type: "AccountBalance", Order: 0, Final: true}
	tResp.Payload, err = json.Marshal(balanceSummary)

	if err != nil {
		c.log.Error("[POLKADOT-CLIENT] Error encoding payload data", zap.Error(err))
	}

	if err := stream.Send(tResp); err != nil {
		c.log.Error("[POLKADOT-CLIENT] Error sending end", zap.Error(err))
	}
}

func (c *Client) getBalanceSummary(ctx context.Context, stream *cStructs.StreamAccess, account string, height uint64) (*structs.GetAccountBalanceResponse, error) {
	accountBalanceResp, err := c.proxy.GetAccountBalance(ctx, account, height)
	if err != nil {
		return nil, err
	}

	return c.abMapper.AccountBalanceMapper(accountBalanceResp, height)
}

// GetLatest returns latest Block's Transactions
func (c *Client) GetLatestMark(ctx context.Context, tr cStructs.TaskRequest, stream *cStructs.StreamAccess) {
	timer := metrics.NewTimer(getLatestDuration)
	defer timer.ObserveDuration()

	ldr := &structs.LatestDataRequest{}
	err := json.Unmarshal(tr.Payload, ldr)
	if err != nil {
		stream.Send(cStructs.TaskResponse{Id: tr.Id, Error: cStructs.TaskError{Msg: "Cannot unmarshal payload"}, Final: true})
	}

	ch := c.gbPool.Get()
	height, err := getLatestHeight(c.serverConn, c.Cache, ch)
	if err != nil {
		stream.Send(cStructs.TaskResponse{
			Id:    tr.Id,
			Error: cStructs.TaskError{Msg: fmt.Sprintf("Could not fetch latest height from proxy: %s", err.Error())},
			Final: true,
		})
		return
	}

	tResp := cStructs.TaskResponse{Id: tr.Id, Type: "LatestMark", Order: 0, Final: true}
	tResp.Payload, err = json.Marshal(structs.LatestDataResponse{
		LastHeight: height,
	})

	if err != nil {
		c.log.Error("[POLKADOT-CLIENT] Error encoding payload data", zap.Error(err))
	}

	if err := stream.Send(tResp); err != nil {
		c.log.Error("[POLKADOT-CLIENT] Error sending end", zap.Error(err))
	}
}

func (c *Client) getLatestBlockHeightRange(ctx context.Context, lastHeight, lastHeightFromProxy uint64) structs.HeightRange {
	if lastHeight == 0 {
		startheight := lastHeightFromProxy - c.maxHeightsToGet
		if startheight > 0 {
			return structs.HeightRange{
				StartHeight: startheight,
				EndHeight:   lastHeightFromProxy,
			}
		}
	}

	if c.maxHeightsToGet < lastHeightFromProxy-lastHeight {
		return structs.HeightRange{
			StartHeight: lastHeightFromProxy - c.maxHeightsToGet,
			EndHeight:   lastHeightFromProxy,
		}
	}

	return structs.HeightRange{
		StartHeight: lastHeight,
		EndHeight:   lastHeightFromProxy,
	}
}

// GetTransactions returns Transactions with given range
func (c *Client) GetTransactions(ctx context.Context, tr cStructs.TaskRequest, stream *cStructs.StreamAccess) {
	timer := metrics.NewTimer(getTransactionDuration)
	defer timer.ObserveDuration()

	var hr structs.HeightRange
	var err error

	if err = json.Unmarshal(tr.Payload, &hr); hr.StartHeight == 0 || hr.EndHeight == 0 {
		err = ErrBadRequest
	}

	if err != nil {
		c.log.Debug("Cannot unmarshal payload", zap.String("contents", string(tr.Payload)))
		stream.Send(cStructs.TaskResponse{
			Id: tr.Id,
			Error: cStructs.TaskError{
				Msg: fmt.Sprintf("Cannot unmarshal payload: %s", err.Error()),
			},
			Final: true,
		})
		return
	}

	if hr.StartHeight > hr.EndHeight {
		c.log.Debug("Bad range, start height is too high", zap.String("contents", string(tr.Payload)))
		stream.Send(cStructs.TaskResponse{
			Id: tr.Id,
			Error: cStructs.TaskError{
				Msg: "Bad range, start height is too high",
			},
			Final: true,
		})
		return
	}

	c.log.Debug("[POLKADOT-CLIENT] Getting Range", zap.Stringer("taskID", tr.Id), zap.Uint64("start", hr.StartHeight), zap.Uint64("end", hr.EndHeight))

	heights, err := c.Reqester.GetRange(ctx, hr)
	resp := &cStructs.TaskResponse{
		Id:    tr.Id,
		Type:  "Heights",
		Final: true,
	}
	if heights.NumberOfHeights > 0 {
		resp.Payload, _ = json.Marshal(heights)
	}

	if err != nil {
		resp.Error = cStructs.TaskError{Msg: err.Error()}
		c.log.Error("[POLKADOT-CLIENT] Error getting range (Get Transactions) ", zap.Error(err), zap.Stringer("taskID", tr.Id))
		if err := stream.Send(*resp); err != nil {
			c.log.Error("[POLKADOT-CLIENT] Error sending message (Get Transactions) ", zap.Error(err), zap.Stringer("taskID", tr.Id))
		}
		return
	}
	if err := stream.Send(*resp); err != nil {
		c.log.Error("[POLKADOT-CLIENT] Error sending message (Get Transactions) ", zap.Error(err), zap.Stringer("taskID", tr.Id))
	}
	c.log.Debug("[POLKADOT-CLIENT] Finished sending all", zap.Stringer("taskID", tr.Id), zap.Any("heights", hr))
}

func (c *Client) BlockAndTx(ctx context.Context, height uint64) (blockWM structs.BlockWithMeta, txsWM []structs.TransactionWithMeta, err error) {
	defer c.log.Sync()
	c.log.Debug("[POLKADOT-CLIENT] Getting height", zap.Uint64("block", height))

	blockWM, txsWM, err = c.blockAndTx(ctx, height)
	if err != nil {
		c.log.Error("[POLKADOT-CLIENT] Error while getting block and transactions", zap.Uint64("block", height), zap.Error(err), zap.Uint64("txs", blockWM.Block.NumberOfTransactions))
		return structs.BlockWithMeta{}, nil, fmt.Errorf("error fetching block and transactions: %d %w ", uint64(height), err)
	}

	hSess, err := c.searchStore.GetSearchSession(ctx)
	if err != nil {
		return structs.BlockWithMeta{}, nil, fmt.Errorf("Error while getting store session: %s", err.Error())
	}

	c.log.Debug("Store block", zap.Uint64("height", height), zap.Uint64("txs", blockWM.Block.NumberOfTransactions))
	if err := hSess.StoreBlocks(ctx, []structs.BlockWithMeta{blockWM}); err != nil {
		return structs.BlockWithMeta{}, nil, fmt.Errorf("Error while storing block: %s", err.Error())
	}

	if len(txsWM) > 0 {
		c.log.Debug("Store transactions", zap.Uint64("height", height), zap.Uint64("txs", blockWM.Block.NumberOfTransactions))
		if err := hSess.StoreTransactions(ctx, txsWM); err != nil {
			return structs.BlockWithMeta{}, nil, fmt.Errorf("Error while storing transactions: %s", err.Error())
		}
	}

	return blockWM, txsWM, nil
}

// GetLatestData returns latest data
func (c *Client) GetLatestData(ctx context.Context, tr cStructs.TaskRequest, stream *cStructs.StreamAccess) {
	timer := metrics.NewTimer(getAccountBalanceDuration)
	defer timer.ObserveDuration()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	blockWM, _, err := c.blockAndTx(ctx, 0)
	if err != nil {
		stream.Send(cStructs.TaskResponse{
			Id: tr.Id,
			Error: cStructs.TaskError{
				Msg: fmt.Sprintf("Could not send head info: %s", err.Error()),
			},
			Final: true,
		})
		return
	}

	b := structs.Block{
		Hash:   blockWM.Block.Hash,
		Height: blockWM.Block.Height,
		Time:   blockWM.Block.Time,
	}

	tResp := cStructs.TaskResponse{Id: tr.Id, Type: "Block", Final: true}
	tResp.Payload, err = json.Marshal(b)

	if err != nil {
		c.log.Error("[POLKADOT-CLIENT] Error encoding payload data", zap.Error(err))
	}

	if err := stream.Send(tResp); err != nil {
		c.log.Error("[POLKADOT-CLIENT] Error sending end", zap.Error(err))
	}
}
