package gosnmp

import (
	"bytes"
)

type bufferPool struct {
	freeList chan *bytes.Buffer
	bufSize  int
	logger   Logger
}

// newBufferPool creates an empty buffer pool with space to hold n buffers of size bufSize
func newBufferPool(n, bufSize int, logger Logger) *bufferPool {
	pool := new(bufferPool)
	pool.freeList = make(chan *bytes.Buffer, n)
	pool.bufSize = bufSize
	pool.logger = logger
	return pool
}

// getBuffer gets a buffer from the pool. If the pool is empty, a new buffer will be allocated
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

// putBuffer returns a buffer to the pool. If the pool is full, the buffer will be garbage collected
func (p *bufferPool) putBuffer(buf *bytes.Buffer) {
	buf.Reset()
	select {
	case p.freeList <- buf:
		// buffer is back on the freelist
	default:
		// freeList is full... buffer will be gc'd
		p.logger.Debugf("Throwing away buffer of size %d", buf.Len())
	}
}
