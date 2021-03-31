package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

var ErrConnectionClosed = errors.New("connection closed")

type JsonRPCRequest struct {
	ID      uint64        `json:"id"`
	JSONRPC string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
}

type JsonRPCSend struct {
	JsonRPCRequest
	RespCH chan Response
}

type JsonRPCResponse struct {
	ID      uint64          `json:"id"`
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result"`
}

type ResponseStore struct {
	ID     uint64 `json:"id"` // originalID
	Type   string
	RespCH chan Response
}

type Response struct {
	ID     uint64
	Error  error
	Type   string
	Result json.RawMessage
}

type Conn struct {
	l        *zap.Logger
	Requests chan JsonRPCSend
}

type LockedResponseMap struct {
	Map map[uint64]ResponseStore
	L   sync.RWMutex
}

func NewConn(l *zap.Logger) *Conn {
	return &Conn{
		l:        l,
		Requests: make(chan JsonRPCSend),
	}
}

// Send is there just because of mock, it doesn't make much sense otherwise
func (conn *Conn) Send(ch chan Response, id uint64, method string, params []interface{}) {
	conn.Requests <- JsonRPCSend{
		RespCH:         ch,
		JsonRPCRequest: JsonRPCRequest{ID: id, Method: method, Params: params},
	}
}

func (conn *Conn) recv(ctx context.Context, c *websocket.Conn, done chan struct{}, resps *LockedResponseMap) {
	defer close(done)
	for {
		_, message, err := c.ReadMessage()
		if err != nil {
			conn.l.Error("error reading next message", zap.Error(err))
			continue
		}

		res := &JsonRPCResponse{}
		err = json.Unmarshal(message, res)
		if err != nil {
			conn.l.Error("error unmarshaling jsonrpc response", zap.Error(err))
			continue
		}

		resps.L.Lock()
		ch := resps.Map[res.ID]
		ch.RespCH <- Response{
			ID:     ch.ID,
			Type:   ch.Type,
			Result: res.Result,
		}
		delete(resps.Map, res.ID)
		resps.L.Unlock()
	}
}

func (conn *Conn) Run(ctx context.Context, addr string) {
	f := make(chan struct{})
	go conn.run(ctx, addr, f)
	select { // reconnects respecting context
	case <-ctx.Done():
		return
	case <-f:
		<-time.After(time.Second)
		go conn.run(ctx, addr, f)
	}
}

func (conn *Conn) run(ctx context.Context, addr string, f chan struct{}) {

	defer conn.l.Sync()
	var nextMessageID uint64

	responseMap := &LockedResponseMap{Map: make(map[uint64]ResponseStore)}

	urlHost := url.URL{Scheme: "ws", Host: addr, Path: "/"}
	conn.l.Info("[API] Connecting to websocket ", zap.String("host", addr))

	c, _, err := websocket.DefaultDialer.DialContext(ctx, urlHost.String(), nil)
	if err != nil {
		conn.l.Error("[API] Error connecting to websocket ", zap.String("host", addr), zap.Error(err))
	}
	defer c.Close()

	done := make(chan struct{})
	go conn.recv(ctx, c, done, responseMap)

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	buff := new(bytes.Buffer)
	enc := json.NewEncoder(buff)
WSLOOP:
	for {
		select {
		case <-ctx.Done():
			err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				conn.l.Error("[API] Error closing websocket ", zap.Error(err))
				break WSLOOP
			}
			select {
			case <-done:
			case <-time.After(time.Second):
			}
			break WSLOOP
		case <-done:
			break WSLOOP
		case req := <-conn.Requests:
			originalID := req.ID
			req.ID = nextMessageID
			if req.JSONRPC == "" {
				req.JSONRPC = "2.0"
			}

			nextMessageID++
			if err := enc.Encode(req.JsonRPCRequest); err != nil {
				req.RespCH <- Response{
					ID:    originalID,
					Type:  req.Method,
					Error: fmt.Errorf("error encoding message: %w ", err),
				}
				continue WSLOOP
			}
			responseMap.L.Lock()
			responseMap.Map[req.ID] = ResponseStore{
				ID:     originalID,
				Type:   req.Method,
				RespCH: req.RespCH,
			}
			responseMap.L.Unlock()
			err = c.WriteMessage(websocket.TextMessage, buff.Bytes())
			buff.Reset()
			if err != nil {
				conn.l.Error("[API] Error sending data websocket ", zap.Error(err))
				break WSLOOP
			}
		}
	}
	responseMap.L.RLock()
	for _, resp := range responseMap.Map {
		resp.RespCH <- Response{
			ID:    resp.ID,
			Type:  resp.Type,
			Error: ErrConnectionClosed,
		}
	}
	responseMap.L.RUnlock()

	conn.l.Info("[API] Websocket listener finished", zap.String("host", addr))
	f <- struct{}{}
}
