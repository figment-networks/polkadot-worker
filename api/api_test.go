package api

import (
	"context"
	"testing"

	"go.uber.org/zap/zaptest"
)

func TestConn_Run(t *testing.T) {

	type args struct {
		addr string
	}
	tests := []struct {
		name string
		args args
	}{
		{name: "ok", args: args{addr: "0.0.0.0:9944"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := NewConn(zaptest.NewLogger(t))
			go conn.Run(context.Background(), tt.args.addr)

			ch := make(chan Response, 10)

			conn.Requests <- JsonRPCSend{
				RespCH: ch,
				JsonRPCRequest: JsonRPCRequest{
					ID:     9,
					Method: "chain_getBlockHash",
					Params: []interface{}{4320198},
				},
			}

			conn.Requests <- JsonRPCSend{
				RespCH: ch,
				JsonRPCRequest: JsonRPCRequest{
					ID:     10,
					Method: "chain_getBlock",
					Params: []interface{}{"0xb715152d0be9c3665ac666427d98f7e6ff014e50e1ec62c369b8ae18337c48b2"},
				},
			}

			conn.Requests <- JsonRPCSend{
				RespCH: ch,
				JsonRPCRequest: JsonRPCRequest{
					ID:     11,
					Method: "state_getMetadata",
					Params: []interface{}{"0xb715152d0be9c3665ac666427d98f7e6ff014e50e1ec62c369b8ae18337c48b2"},
				},
			}

			var i int
			for {
				select {
				case a := <-ch:
					t.Log(a.Type)
					if i == 2 {
						return
					}
					i++
				}
			}
		})
	}
}
