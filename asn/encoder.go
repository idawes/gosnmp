package asn

import (
	"bytes"
	"container/list"
	. "github.com/idawes/gosnmp/common"
	"runtime"
)

type BerEncoderFactory struct {
	bufPool *bufferPool
	logger  Logger
}

func NewBerEncoderFactory(logger Logger) *BerEncoderFactory {
	factory := new(BerEncoderFactory)
	// Encoders are typically used and destroyed in short order, so we should only have a few active at any time. Each encoder may use quite a few small
	// temporary buffers during the encoding process though. Here we're setting things up for NumCpu * 2 encoders to be able to use 200 buffers each.
	factory.bufPool = newBufferPool(runtime.NumCPU()*2*200, 64, logger)
	factory.logger = logger
	return factory
}

type BerEncoder struct {
	bufChain *list.List
	bufPool  *bufferPool
	logger   Logger
}

func (factory *BerEncoderFactory) newBerEncoder() *BerEncoder {
	encoder := new(BerEncoder)
	encoder.bufChain = list.New()
	encoder.bufPool = factory.bufPool
	encoder.logger = factory.logger
	return encoder
}

func (encoder *BerEncoder) serialize() []byte {
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

func (encoder *BerEncoder) destroy() {
	for e := encoder.bufChain.Front(); e != nil; e = e.Next() {
		buf := e.Value.(*bytes.Buffer)
		encoder.bufPool.putBuffer(buf)
	}
	encoder.bufChain.Init() // make sure to release references
}

func (e *BerEncoder) append() *bytes.Buffer {
	buf := e.bufPool.getBuffer()
	e.bufChain.PushBack(buf)
	return buf
}

type BerHeader struct {
	buf *bytes.Buffer
}

func (encoder *BerEncoder) newHeader(blockType SnmpBlockType) *BerHeader {
	h := BerHeader{buf: encoder.append()}
	h.buf.WriteByte(byte(blockType))
	return &h
}

func (h *BerHeader) setContentLength(contentLength int) (headerLength int, blockLength int) {
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
