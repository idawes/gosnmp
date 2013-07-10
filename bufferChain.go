package snmp_go

import (
	"bytes"
	"container/list"
)

type bufferChain struct {
	chain   *list.List
	bufPool *bufferPool
}

func newBufferChain(bufPool *bufferPool) *bufferChain {
	c := new(bufferChain)
	c.chain = list.New()
	c.bufPool = bufPool
	return c
}

func (bc *bufferChain) addBufToHead() *bytes.Buffer {
	buf := bc.bufPool.getBuffer()
	bc.chain.PushFront(buf)
	return buf
}

func (bc *bufferChain) addBufToTail() *bytes.Buffer {
	buf := bc.bufPool.getBuffer()
	bc.chain.PushBack(buf)
	return buf
}

func (bc *bufferChain) collapse() []byte {
	totalLen := 0
	for e := bc.chain.Front(); e != nil; e = e.Next() {
		buf := e.Value.(*bytes.Buffer)
		totalLen += buf.Len()
	}
	serializedMsg := bytes.NewBuffer(make([]byte, 0, totalLen))
	for e := bc.chain.Front(); e != nil; e = e.Next() {
		buf := e.Value.(*bytes.Buffer)
		buf.WriteTo(serializedMsg)
	}
	return serializedMsg.Bytes()
}

func (bc *bufferChain) destroy() {
	for e := bc.chain.Front(); e != nil; e = e.Next() {
		buf := e.Value.(*bytes.Buffer)
		bc.bufPool.putBuffer(buf)
	}
	bc.chain.Init() // make sure to release references
}
