package indexer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/figment-networks/indexer-manager/structs"
	cStructs "github.com/figment-networks/indexer-manager/worker/connectivity/structs"
	"github.com/figment-networks/indexing-engine/metrics"
	"github.com/figment-networks/polkadot-worker/mapper"
	"github.com/figment-networks/polkadot-worker/proxy"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

const page = 100

var (
	// ErrBadRequest is returned when cannot unmarshal message
	ErrBadRequest = errors.New("bad request")

	getAccountBalanceDuration *metrics.GroupObserver
	getTransactionDuration    *metrics.GroupObserver
	getLatestDuration         *metrics.GroupObserver
)

// Client connecting to indexer-manager
type Client struct {
	chainID         string
	currency        string
	exp             int
	maxHeightsToGet uint64

	log     *zap.Logger
	proxy   proxy.ClientIface
	sLock   sync.Mutex
	streams map[uuid.UUID]*cStructs.StreamAccess

	abMapper *mapper.AccountBalanceMapper
	trMapper *mapper.TransactionMapper
}

// NewClient is a indexer-manager Client constructor
func NewClient(log *zap.Logger, proxy proxy.ClientIface, exp int, maxHeightsToGet uint64, chainID, currency string) *Client {
	getAccountBalanceDuration = endpointDuration.WithLabels("getAccountBalance")
	getTransactionDuration = endpointDuration.WithLabels("getTransactions")
	getLatestDuration = endpointDuration.WithLabels("getLatest")

	return &Client{
		chainID:         chainID,
		currency:        currency,
		exp:             exp,
		maxHeightsToGet: maxHeightsToGet,
		log:             log,
		proxy:           proxy,
		streams:         make(map[uuid.UUID]*cStructs.StreamAccess),
		abMapper:        mapper.NewAccountBalanceMapper(exp, currency),
		trMapper:        mapper.NewTransactionMapper(exp, chainID, currency),
	}
}

// RegisterStream adds new listeners to the stream
func (c *Client) RegisterStream(ctx context.Context, stream *cStructs.StreamAccess) error {
	c.log.Debug("Register indexer-manager client stream", zap.Stringer("streamID", stream.StreamID))

	c.sLock.Lock()
	defer c.sLock.Unlock()
	c.streams[stream.StreamID] = stream

	for i := 0; i < 20; i++ {
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

			ctxWithTimeout, cancel := context.WithTimeout(ctx, 5*time.Minute)
			defer cancel()

			switch taskRequest.Type {
			case structs.ReqIDAccountBalance:
				c.GetAccountBalance(ctxWithTimeout, taskRequest, stream)
			case structs.ReqIDGetTransactions:
				c.GetTransactions(ctxWithTimeout, taskRequest, stream)
			case structs.ReqIDLatestData:
				c.GetLatest(ctxWithTimeout, taskRequest, stream)
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

	out := make(chan cStructs.OutResp, 2)
	fin := make(chan bool, 2)

	go c.sendRespLoop(ctx, tr.Id, out, stream, fin)

	if err := c.sendAccountBalance(ctx, hr.Account, hr.Height, out); err != nil {
		stream.Send(cStructs.TaskResponse{
			Id: tr.Id,
			Error: cStructs.TaskError{
				Msg: fmt.Sprintf("Could not send Account Balance: %s", err.Error()),
			},
			Final: true,
		})
		close(out)
		return
	}

	close(out)

	for {
		select {
		case <-ctx.Done():
			c.log.Debug("Context done", zap.Stringer("taskID", tr.Id))
			return
		case <-fin:
			c.log.Debug("Finished sending all", zap.Stringer("taskID", tr.Id))
			return
		}
	}
}

func (c *Client) sendAccountBalance(ctx context.Context, account string, height uint64, out chan cStructs.OutResp) error {
	accountBalanceResp, err := c.proxy.GetAccountBalance(ctx, account, height)
	if err != nil {
		return err
	}

	balanceSummary, err := c.abMapper.AccountBalanceMapper(accountBalanceResp, height)
	if err != nil {
		return err
	}

	out <- cStructs.OutResp{
		Type:    "AccountBalance",
		Payload: *balanceSummary,
	}

	return nil
}

// GetLatest returns latest Block's Transactions
func (c *Client) GetLatest(ctx context.Context, tr cStructs.TaskRequest, stream *cStructs.StreamAccess) {
	timer := metrics.NewTimer(getLatestDuration)
	defer timer.ObserveDuration()

	var ldr structs.LatestDataRequest
	var err error

	if err = json.Unmarshal(tr.Payload, &ldr); ldr.LastHeight == 0 {
		err = ErrBadRequest
	}

	if err != nil {
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

	out := make(chan cStructs.OutResp, page*2+1)
	fin := make(chan bool, 2)

	go c.sendRespLoop(ctx, tr.Id, out, stream, fin)

	head, err := c.proxy.GetHead(ctx)
	if err != nil {
		stream.Send(cStructs.TaskResponse{
			Id:    tr.Id,
			Error: cStructs.TaskError{Msg: fmt.Sprintf("Could not fetch head from proxy: %s", err.Error())},
			Final: true,
		})
		return
	}

	hr := c.getLatestBlockHeightRange(ctx, ldr.LastHeight, uint64(head.GetHeight()))

	if err := sendTransactionsInRange(ctx, c.log, c, hr, out); err != nil {
		stream.Send(cStructs.TaskResponse{
			Id:    tr.Id,
			Error: cStructs.TaskError{Msg: fmt.Sprintf("Error while getting Transactions with given range: %s", err.Error())},
			Final: true,
		})
		close(out)
		return
	}

	c.log.Debug("Received all", zap.Stringer("taskID", tr.Id))
	close(out)

	for {
		select {
		case <-ctx.Done():
			c.log.Debug("Context done", zap.Stringer("taskID", tr.Id))
			return
		case <-fin:
			c.log.Debug("Finished sending all", zap.Stringer("taskID", tr.Id))
			return
		}
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
		c.log.Debug("Cannot unmarshal payload", zap.String("contents", string(tr.Payload)))
		stream.Send(cStructs.TaskResponse{
			Id: tr.Id,
			Error: cStructs.TaskError{
				Msg: "Bad range, start height is too high",
			},
			Final: true,
		})
		return
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	out := make(chan cStructs.OutResp, page*2+1)
	fin := make(chan bool, 2)

	go c.sendRespLoop(ctx, tr.Id, out, stream, fin)

	if err := sendTransactionsInRange(ctx, c.log, c, hr, out); err != nil {
		stream.Send(cStructs.TaskResponse{
			Id: tr.Id,
			Error: cStructs.TaskError{
				Msg: fmt.Sprintf("Error while sending Transactions with given range: %s", err.Error()),
			},
			Final: true,
		})
		close(out)
		return
	}

	c.log.Debug("Received all", zap.Stringer("taskID", tr.Id))
	close(out)

	for {
		select {
		case <-ctx.Done():
			c.log.Debug("Context done", zap.Stringer("taskID", tr.Id))
			return
		case <-fin:
			c.log.Debug("Finished sending all", zap.Stringer("taskID", tr.Id))
			return
		}
	}
}

func (c *Client) sendRespLoop(ctx context.Context, id uuid.UUID, in <-chan cStructs.OutResp, stream *cStructs.StreamAccess, fin chan bool) {
	var ctxDone bool
	order := uint64(0)

SendLoop:
	for {
		select {
		case <-ctx.Done():
			ctxDone = true
			break SendLoop
		case t, ok := <-in:
			if !ok {
				break SendLoop
			}

			c.sendResp(id, t.Type, t.Payload, order, stream)
			order++
		}
	}

	if err := stream.Send(cStructs.TaskResponse{
		Id:    id,
		Type:  "END",
		Order: order,
		Final: true,
	}); err != nil {
		c.log.Error("Error while sending end response %w", zap.Error(err))
	}

	if fin != nil {
		if !ctxDone {
			fin <- true
		}
		close(fin)
	}
}

func (c *Client) sendResp(id uuid.UUID, taskType string, payload interface{}, order uint64, stream *cStructs.StreamAccess) {
	var buffer bytes.Buffer
	enc := json.NewEncoder(&buffer)
	if err := enc.Encode(payload); err != nil {
		c.log.Error("Cannot encode payload %w", zap.Error(err))
	}

	tr := cStructs.TaskResponse{
		Id:      id,
		Type:    taskType,
		Order:   order,
		Payload: make([]byte, buffer.Len()),
	}
	buffer.Read(tr.Payload)

	if err := stream.Send(tr); err != nil {
		c.log.Error("Error while sending response %w", zap.Error(err))
	}
}

type hBTx struct {
	Height uint64
	Last   bool
	Ch     chan cStructs.OutResp
}

// getRange gets given range of blocks and transactions
func sendTransactionsInRange(ctx context.Context, logger *zap.Logger, client *Client, hr structs.HeightRange, out chan cStructs.OutResp) (err error) {
	defer logger.Sync()

	chIn := oHBTxPool.Get()
	chOut := oHBTxPool.Get()

	errored := make(chan bool, 7)
	defer close(errored)

	wg := &sync.WaitGroup{}
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go asyncBlockAndTx(ctx, logger, wg, client, chIn)
	}
	go populateRange(chIn, chOut, hr, errored)

RANGE_LOOP:
	for {
		select {
		// (lukanus): add timeout
		case o := <-chOut:
			if o.Last {
				logger.Debug("[CLIENT] Finished sending height", zap.Uint64("height", o.Height))
				break RANGE_LOOP
			}

		INNER_LOOP:
			for resp := range o.Ch {
				switch resp.Type {
				case "Partial":
					break INNER_LOOP
				case "Error":
					errored <- true // (lukanus): to close publisher and asyncBlockAndTx
					err = resp.Error
					out <- resp
					break INNER_LOOP
				default:
					out <- resp
				}
			}
			oRespPool.Put(o.Ch)
		}
	}

	if err != nil { // (lukanus): discard everything on error, after error
		wg.Wait() // (lukanus): make sure there are no outstanding producers
	PURIFY_CHANNELS:
		for {
			select {
			case o := <-chOut:
				if o.Ch != nil {
				PURIFY_INNER_CHANNELS:
					for {
						select {
						case <-o.Ch:
						default:
							break PURIFY_INNER_CHANNELS
						}
					}
				}
				oRespPool.Put(o.Ch)
			default:
				break PURIFY_CHANNELS
			}
		}
	}
	oHBTxPool.Put(chOut)
	return err
}

func populateRange(in, out chan hBTx, hr structs.HeightRange, er chan bool) {
	height := hr.StartHeight

	for {
		hBTxO := hBTx{Height: height, Ch: oRespPool.Get()}
		select {
		case out <- hBTxO:
		case <-er:
			break
		}

		select {
		case in <- hBTxO:
		case <-er:
			break
		}

		height++
		if height > hr.EndHeight {
			select {
			case out <- hBTx{Last: true}:
			case <-er:
			}
			break
		}

	}
	close(in)
}

func asyncBlockAndTx(ctx context.Context, logger *zap.Logger, wg *sync.WaitGroup, client *Client, cinn chan hBTx) {
	defer wg.Done()
	for in := range cinn {
		b, txs, err := blockAndTx(ctx, logger, client, in.Height)
		if err != nil {
			in.Ch <- cStructs.OutResp{
				Error: err,
				Type:  "Error",
			}
			return
		}
		in.Ch <- cStructs.OutResp{
			Type:    "Block",
			Payload: b,
		}
		if txs != nil {
			for _, t := range txs {
				in.Ch <- cStructs.OutResp{
					Type:    "Transaction",
					Payload: t,
				}
			}
		}

		in.Ch <- cStructs.OutResp{
			Type: "Partial",
		}
	}
}

func blockAndTx(ctx context.Context, logger *zap.Logger, c *Client, height uint64) (block *structs.Block, transactions []*structs.Transaction, err error) {
	blResp, err := c.proxy.GetBlockByHeight(ctx, height)
	if err != nil {
		return nil, nil, err
	}

	trResp, err := c.proxy.GetTransactionsByHeight(ctx, height)
	if err != nil {
		return nil, nil, err
	}

	if trResp == nil {
		return mapper.BlockMapper(blResp, c.chainID, 0), nil, nil
	}

	block = mapper.BlockMapper(blResp, c.chainID, uint64(len(trResp.Transactions)))

	evResp, err := c.proxy.GetEventsByHeight(ctx, height)
	if err != nil {
		return nil, nil, err
	}

	metaResp, err := c.proxy.GetMetaByHeight(ctx, height)
	if err != nil {
		return nil, nil, err
	}

	if transactions, err = c.trMapper.TransactionsMapper(c.log, blResp, evResp, metaResp, trResp); err != nil {
		return nil, nil, err
	}

	return
}

func (c *Client) wrapErrorsFromChan(errChan chan error) error {
	var errors []error
	for err := range errChan {
		if err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		errStr := ""
		for _, err := range errors {
			errStr += err.Error() + " , "
		}
		return fmt.Errorf(fmt.Sprintf("%s", errStr))
	}

	return nil
}
