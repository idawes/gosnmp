package gosnmp

import (
	"bytes"
	"container/list"
	"runtime"
)

type berEncoderFactory struct {
	bufPool *bufferPool
	logger  Logger
}

func newberEncoderFactory(logger Logger) *berEncoderFactory {
	factory := new(berEncoderFactory)
	// Encoders are typically used and destroyed in short order, so we should only have a few active at any time. Each encoder may use quite a few small
	// temporary buffers during the encoding process though. Here we're setting things up for NumCpu * 2 encoders to be able to use 200 buffers each.
	factory.bufPool = newBufferPool(runtime.NumCPU()*2*200, 64, logger)
	factory.logger = logger
	return factory
}

type berEncoder struct {
	bufChain *list.List
	bufPool  *bufferPool
	logger   Logger
}

func (factory *berEncoderFactory) newberEncoder() *berEncoder {
	encoder := new(berEncoder)
	encoder.bufChain = list.New()
	encoder.bufPool = factory.bufPool
	encoder.logger = factory.logger
	return encoder
}

func (encoder *berEncoder) serialize() []byte {
	totalLen := 0
	for e := encoder.bufChain.Front(); e != nil; e = e.Next() {
		buf := e.Value.(*bytes.Buffer)
		totalLen += buf.Len()
	}
	serializedMsg := bytes.NewBuffer(make([]byte, 0, totalLen))
	for e := encoder.bufChain.Front(); e != nil; e = e.Next() {
		buf := e.Value.(*bytes.Buffer)
		buf.WriteTo(serializedMsg)
	}
	return serializedMsg.Bytes()
}

func (encoder *berEncoder) destroy() {
	for e := encoder.bufChain.Front(); e != nil; e = e.Next() {
		buf := e.Value.(*bytes.Buffer)
		encoder.bufPool.putBuffer(buf)
	}
	encoder.bufChain.Init() // make sure to release references
}

func (e *berEncoder) append() *bytes.Buffer {
	buf := e.bufPool.getBuffer()
	e.bufChain.PushBack(buf)
	return buf
}

type berHeader struct {
	buf *bytes.Buffer
}

func (encoder *berEncoder) newHeader(blockType snmpBlockType) *berHeader {
	h := berHeader{buf: encoder.append()}
	h.buf.WriteByte(byte(blockType))
	return &h
}

func (h *berHeader) setContentLength(contentLength int) (headerLength int, blockLength int) {
	if contentLength < 127 {
		h.buf.WriteByte(byte(contentLength))
	} else {
		n := calculateLengthLen(contentLength)
		h.buf.WriteByte(0x80 | n)
		for ; n > 0; n-- {
			h.buf.WriteByte(byte(contentLength >> uint((n-1)*8)))
		}
	}
	headerLength = h.buf.Len()
	blockLength = headerLength + contentLength
	return
}

func calculateLengthLen(l int) byte {
	numBytes := 1
	for ; l > 255; l >>= 8 {
		numBytes++
	}
	return byte(numBytes)
}
