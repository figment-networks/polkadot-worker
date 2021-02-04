package indexer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/figment-networks/indexing-engine/metrics"
	"github.com/figment-networks/polkadot-worker/worker/mapper"
	"github.com/figment-networks/polkadot-worker/worker/proxy"

	"github.com/figment-networks/indexer-manager/structs"
	cStructs "github.com/figment-networks/indexer-manager/worker/connectivity/structs"

	"github.com/google/uuid"
	"github.com/prometheus/common/log"
	"go.uber.org/zap"
)

var errBadRequest = errors.New("bad request")
var (
	getTransactionDuration *metrics.GroupObserver
	getLatestDuration      *metrics.GroupObserver
)

// Client connecting to indexer-manager
type Client struct {
	sLock   sync.Mutex
	streams map[uuid.UUID]*cStructs.StreamAccess

	chainID string
	log     *zap.SugaredLogger
	page    uint64
	proxy   proxy.ClientIface
}

// NewClient is a indexer-manager Client constructor
func NewClient(bigPage uint64, chainID string, log *zap.SugaredLogger, page uint64, proxy *proxy.Client) *Client {
	getTransactionDuration = endpointDuration.WithLabels("getTransactions")
	getLatestDuration = endpointDuration.WithLabels("getLatest")

	return &Client{
		chainID: chainID,
		log:     log,
		page:    page,
		proxy:   proxy,
		streams: make(map[uuid.UUID]*cStructs.StreamAccess),
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

// CloseStream cloes connection with indexer-manager
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
			c.log.Debug("Recived task request", zap.Stringer("taskID", taskRequest.Id), zap.String("type", taskRequest.Type))

			ctxWithTimeout, cancel := context.WithTimeout(ctx, 5*time.Minute)
			defer cancel()

			switch taskRequest.Type {
			case structs.ReqIDGetTransactions:
				c.GetTransactions(ctxWithTimeout, taskRequest, stream)
			case structs.ReqIDLatestData:
				c.GetLatest(ctxWithTimeout, taskRequest, stream)
			default:
				stream.Send(cStructs.TaskResponse{
					Id:    taskRequest.Id,
					Error: cStructs.TaskError{Msg: fmt.Sprintf("Unknown request %s", taskRequest.Type)},
					Final: true,
				})
			}
		}
	}
}

// GetLatest returns latest Block's Transactions
func (c *Client) GetLatest(ctx context.Context, tr cStructs.TaskRequest, stream *cStructs.StreamAccess) {
	timer := metrics.NewTimer(getLatestDuration)
	defer timer.ObserveDuration()

	var ldr structs.LatestDataRequest
	var err error

	if err = json.Unmarshal(tr.Payload, &ldr); ldr.LastHeight == 0 {
		err = errBadRequest
	}

	if err != nil {
		stream.Send(cStructs.TaskResponse{
			Id:    tr.Id,
			Error: cStructs.TaskError{Msg: fmt.Sprintf("Cannot unmarshal payload: %s", err.Error())},
			Final: true,
		})
		return
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errChan := make(chan error, 1)
	out := make(chan cStructs.OutResp, c.page)
	fin := make(chan bool, 2)

	go c.sendRespLoop(ctx, tr.Id, out, stream, fin)

	c.getTransactionsWrapped(ctx, out, ldr.LastHeight, nil, errChan)

	close(errChan)

	if err := c.wrapErrorsFromChan(errChan); err != nil {
		stream.Send(cStructs.TaskResponse{
			Id:    tr.Id,
			Error: cStructs.TaskError{Msg: fmt.Sprintf("Could not fetch latest transactions: %s", err.Error())},
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

// GetTransactions returns Transactions with given range
func (c *Client) GetTransactions(ctx context.Context, tr cStructs.TaskRequest, stream *cStructs.StreamAccess) {
	timer := metrics.NewTimer(getTransactionDuration)
	defer timer.ObserveDuration()

	var hr structs.HeightRange
	var err error

	if err = json.Unmarshal(tr.Payload, &hr); hr.StartHeight == 0 || hr.EndHeight == 0 {
		err = errBadRequest
	}

	if err != nil {
		c.log.Debug("Cannot unmarshal payload", zap.String("contents", string(tr.Payload)))
		stream.Send(cStructs.TaskResponse{
			Id:    tr.Id,
			Error: cStructs.TaskError{Msg: fmt.Sprintf("Cannot unmarshal payload: %s", err.Error())},
			Final: true,
		})
		return
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	out := make(chan cStructs.OutResp, c.page)
	fin := make(chan bool, 1)

	go c.sendRespLoop(ctx, tr.Id, out, stream, fin)

	var i uint64
	for {
		hrInerr := structs.HeightRange{
			StartHeight: hr.StartHeight + i*c.page,
			EndHeight:   hr.StartHeight + i*c.page + c.page - 1,
		}

		if hrInerr.EndHeight > hr.EndHeight {
			hrInerr.EndHeight = hr.EndHeight
		}

		if err := c.getRange(ctx, hrInerr, out); err != nil {
			stream.Send(cStructs.TaskResponse{
				Id:    tr.Id,
				Error: cStructs.TaskError{Msg: fmt.Sprintf("Error while getting Transactions with given range: %s", err.Error())},
				Final: true,
			})
			break
		}

		i++
		if hrInerr.EndHeight == hr.EndHeight {
			break
		}
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
			if !ok && t.Type == "" {
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
		log.Error("Error while sending end response", zap.Error(err))
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
		log.Error("Cannot encode payload", zap.Error(err))
	}

	tr := cStructs.TaskResponse{
		Id:      id,
		Type:    taskType,
		Order:   order,
		Payload: make([]byte, buffer.Len()),
	}

	buffer.Read(tr.Payload)

	if err := stream.Send(tr); err != nil {
		log.Error("Error while sending response", zap.Error(err))
	}
}

func (c *Client) getRange(ctx context.Context, hr structs.HeightRange, out chan cStructs.OutResp) error {
	var wg sync.WaitGroup
	actualHeight := hr.StartHeight

	defer c.log.Sync()

	count := int(hr.EndHeight - hr.StartHeight + 1)
	wg.Add(count)
	errChan := make(chan error, count)

	for {
		c.log.Debug("Getting transactions", zap.Uint64("height", actualHeight))

		c.getTransactionsWrapped(ctx, out, actualHeight, &wg, errChan)

		if actualHeight == hr.EndHeight {
			break
		}

		actualHeight++
	}

	wg.Wait()
	close(errChan)

	if err := c.wrapErrorsFromChan(errChan); err != nil {
		return err
	}

	return nil
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
		return fmt.Errorf(fmt.Sprintf("Error while getting transactions: %s", errStr))
	}

	return nil
}

func (c *Client) getTransactionsWrapped(ctx context.Context, out chan cStructs.OutResp, height uint64, wg *sync.WaitGroup, err chan error) {
	if transactions := c.getTransactions(ctx, out, height, err); transactions != nil {
		for _, transaction := range transactions {
			t := *transaction
			out <- cStructs.OutResp{
				Type:    "Transaction",
				Payload: t,
			}
		}
	}
	if wg != nil {
		wg.Done()
	}
}

func (c *Client) getTransactions(ctx context.Context, out chan cStructs.OutResp, height uint64, err chan error) []*structs.Transaction {
	block, e := c.proxy.GetBlockByHeight(ctx, height)
	if e != nil {
		err <- e
		ctx.Done()
		return nil
	}

	transactions, e := c.proxy.GetTransactionByHeight(ctx, height)
	if e != nil {
		err <- e
		ctx.Done()
		return nil
	}

	if transactions == nil {
		out <- cStructs.OutResp{
			Type:    "Block",
			Payload: mapper.BlockMapper(block, c.chainID, 0),
		}
		return nil
	}

	out <- cStructs.OutResp{
		Type:    "Block",
		Payload: mapper.BlockMapper(block, c.chainID, uint64(len(transactions.Transactions))),
	}

	events, e := c.proxy.GetEventByHeight(ctx, height)
	if e != nil {
		err <- e
		ctx.Done()
		return nil
	}

	transactionMapped, e := mapper.TransactionMapper(block, c.chainID, events, transactions)
	if e != nil {
		err <- e
		ctx.Done()
		return nil
	}

	return transactionMapped
}
