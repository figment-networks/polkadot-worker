package proxy

import (
	"sync"

	"github.com/figment-networks/polkadothub-proxy/grpc/account/accountpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/block/blockpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/chain/chainpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/decode/decodepb"
	"github.com/figment-networks/polkadothub-proxy/grpc/event/eventpb"
	"github.com/figment-networks/polkadothub-proxy/grpc/transaction/transactionpb"
	"google.golang.org/grpc"
)

type GRPConnectionsIface interface {
	Close()
	Add(*grpc.ClientConn)
	GetNextAccountClient() accountpb.AccountServiceClient
	GetNextBlockClient() blockpb.BlockServiceClient
	GetNextChainClient() chainpb.ChainServiceClient
	GetNextEventServiceClient() eventpb.EventServiceClient
	GetNextTransactionServiceClient() transactionpb.TransactionServiceClient
	GetNextDecodeServiceClient() decodepb.DecodeServiceClient
}

type GRPConnections struct {
	next int
	len  int
	Conn []*grpc.ClientConn
	lock sync.Mutex

	AccountClient            []accountpb.AccountServiceClient
	BlockClient              []blockpb.BlockServiceClient
	ChainClient              []chainpb.ChainServiceClient
	EventServiceClient       []eventpb.EventServiceClient
	TransactionServiceClient []transactionpb.TransactionServiceClient
	DecodeServiceClient      []decodepb.DecodeServiceClient
}

func NewGRPConnections() *GRPConnections {
	return &GRPConnections{}
}

func (c *GRPConnections) inc() {
	if c.next == c.len-1 {
		c.next = 0
	} else {
		c.next++
	}
}
func (c *GRPConnections) Close() {
	c.lock.Lock()
	defer c.lock.Unlock()
	for _, conn := range c.Conn {
		conn.Close()
	}
}

func (c *GRPConnections) Add(new *grpc.ClientConn) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.Conn = append(c.Conn, new)
	c.len++
	c.AccountClient = append(c.AccountClient, accountpb.NewAccountServiceClient(new))
	c.BlockClient = append(c.BlockClient, blockpb.NewBlockServiceClient(new))
	c.ChainClient = append(c.ChainClient, chainpb.NewChainServiceClient(new))
	c.EventServiceClient = append(c.EventServiceClient, eventpb.NewEventServiceClient(new))
	c.TransactionServiceClient = append(c.TransactionServiceClient, transactionpb.NewTransactionServiceClient(new))
	c.DecodeServiceClient = append(c.DecodeServiceClient, decodepb.NewDecodeServiceClient(new))
}

func (c *GRPConnections) GetNextAccountClient() (ac accountpb.AccountServiceClient) {
	c.lock.Lock()
	defer c.lock.Unlock()

	ac = c.AccountClient[c.next]
	c.inc()
	return ac
}

func (c *GRPConnections) GetNextBlockClient() (bc blockpb.BlockServiceClient) {
	c.lock.Lock()
	defer c.lock.Unlock()

	bc = c.BlockClient[c.next]
	c.inc()
	return bc
}

func (c *GRPConnections) GetNextChainClient() (cc chainpb.ChainServiceClient) {
	c.lock.Lock()
	defer c.lock.Unlock()

	cc = c.ChainClient[c.next]
	c.inc()
	return cc
}

func (c *GRPConnections) GetNextEventServiceClient() (esc eventpb.EventServiceClient) {
	c.lock.Lock()
	defer c.lock.Unlock()

	esc = c.EventServiceClient[c.next]
	c.inc()
	return esc
}

func (c *GRPConnections) GetNextTransactionServiceClient() (tsc transactionpb.TransactionServiceClient) {
	c.lock.Lock()
	defer c.lock.Unlock()

	tsc = c.TransactionServiceClient[c.next]
	c.inc()
	return tsc
}

func (c *GRPConnections) GetNextDecodeServiceClient() (bc decodepb.DecodeServiceClient) {
	c.lock.Lock()
	defer c.lock.Unlock()

	bc = c.DecodeServiceClient[c.next]
	c.inc()
	return bc
}
