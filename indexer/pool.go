package indexer

import (
	"sync"

	cStructs "github.com/figment-networks/indexer-manager/worker/connectivity/structs"
)

var (
	oRespPool = NewOutRespPool(20)
	oHBTxPool = NewHBTxPool(20)
)

type outRespPool struct {
	stor chan chan cStructs.OutResp
	lock *sync.Mutex
}

func NewOutRespPool(cap int) *outRespPool {
	return &outRespPool{
		stor: make(chan chan cStructs.OutResp, cap),
		lock: &sync.Mutex{},
	}
}

func (o *outRespPool) Get() chan cStructs.OutResp {
	o.lock.Lock()
	defer o.lock.Unlock()
	select {
	case a := <-o.stor:
		// (lukanus): better safe than sorry
		outRespDrain(a)
		return a
	default:
	}

	return make(chan cStructs.OutResp, 1000)
}

func (o *outRespPool) Put(or chan cStructs.OutResp) {
	o.lock.Lock()
	defer o.lock.Unlock()
	select {
	case o.stor <- or:
	default:
		close(or)
	}

	return
}

type hBTxPool struct {
	stor chan chan hBTx
	lock *sync.Mutex
}

func NewHBTxPool(cap int) *hBTxPool {
	return &hBTxPool{
		stor: make(chan chan hBTx, cap),
		lock: &sync.Mutex{},
	}
}

func (o *hBTxPool) Get() chan hBTx {
	o.lock.Lock()
	defer o.lock.Unlock()
	select {
	case a := <-o.stor:
		// (lukanus): better safe than sorry
		hBTxDrain(a)
		return a
	default:
	}

	return make(chan hBTx, 10)
}

func (o *hBTxPool) Put(or chan hBTx) {
	o.lock.Lock()
	defer o.lock.Unlock()
	select {
	case o.stor <- or:
	default:
		close(or)
	}

	return
}

func hBTxDrain(c chan hBTx) {
	for {
		select {
		case <-c:
		default:
			return
		}
	}
}

func outRespDrain(c chan cStructs.OutResp) {
	for {
		select {
		case <-c:
		default:
			return
		}
	}
}
