package api

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

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

func NewConn(l *zap.Logger) *Conn {
	return &Conn{
		l:        l,
		Requests: make(chan JsonRPCSend),
	}
}

func (conn *Conn) Run(ctx context.Context, addr string) {

	var nextMessageID uint64

	responseMap := make(map[uint64]ResponseStore)

	urlHost := url.URL{Scheme: "ws", Host: addr, Path: "/"}
	conn.l.Info("[API] Connecting to websocket ", zap.String("host", addr))

	c, _, err := websocket.DefaultDialer.DialContext(ctx, urlHost.String(), nil)
	if err != nil {
		conn.l.Error("[API] Error connecting to websocket ", zap.String("host", addr), zap.Error(err))
	}
	defer c.Close()

	done := make(chan struct{})

	go func(ctx context.Context) {
		defer close(done)
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				return
			}

			res := &JsonRPCResponse{}
			err = json.Unmarshal(message, res)
			if err != nil {
				log.Println("err", err)
			}

			ch := responseMap[res.ID]
			ch.RespCH <- Response{
				ID:     ch.ID,
				Type:   ch.Type,
				Result: res.Result,
			}
			delete(responseMap, res.ID)
		}
	}(ctx)

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	buff := new(bytes.Buffer)
	enc := json.NewEncoder(buff)
	for {
		select {
		case <-ctx.Done():
			err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				conn.l.Error("[API] Error closing websocket ", zap.Error(err))
				return
			}
			select {
			case <-done:
			case <-time.After(time.Second):
			}
			return
		case <-done:
			return
		case req := <-conn.Requests:
			originalID := req.ID
			req.ID = nextMessageID
			if req.JSONRPC == "" {
				req.JSONRPC = "2.0"
			}

			nextMessageID++
			if err := enc.Encode(req.JsonRPCRequest); err != nil {
				return
			}
			responseMap[req.ID] = ResponseStore{
				ID:     originalID,
				Type:   req.Method,
				RespCH: req.RespCH,
			}
			err = c.WriteMessage(websocket.TextMessage, buff.Bytes())
			buff.Reset()
			if err != nil {
				conn.l.Error("[API] Error sending data websocket ", zap.Error(err))
				return
			}
		}
	}
	// TODO(lukanus): send errors on all open
}
