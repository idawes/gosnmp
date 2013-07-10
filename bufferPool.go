package snmp_go

import (
	"bytes"
)

type bufferPool struct {
	freeList chan *bytes.Buffer
	bufSize  int
}

// NewBufferPool creates an empty buffer pool with space to hold n buffers of size bufSize
func newBufferPool(n, bufSize int) *bufferPool {
	pool := new(bufferPool)
	pool.freeList = make(chan *bytes.Buffer, n)
	pool.bufSize = bufSize
	return pool
}

// GetBuffer gets a buffer from the pool. If the pool is empty, a new buffer will be allocated
func (p *bufferPool) getBuffer() *bytes.Buffer {
	var buf *bytes.Buffer
	select {
	case buf = <-p.freeList:
		// we got a buffer from the free list
	default:
		// free list doesn't have a buffer for us... create a new one.
		if p.bufSize <= 64 {
			// basic buffer has a built in starter... no need to allocate our own.
			buf = new(bytes.Buffer)
		} else {
			buf = bytes.NewBuffer(make([]byte, 0, p.bufSize))
		}
	}
	return buf
}

// ReleaseBuffer returns a buffer to the pool. If the pool is full, the buffer will be garbage collected
func (p *bufferPool) putBuffer(buf *bytes.Buffer) {
	buf.Reset()
	select {
	case p.freeList <- buf:
		// buffer is back on the freelist
	default:
		// freeList is full... buffer will be gc'd
	}
}
