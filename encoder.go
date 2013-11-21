package snmp_go

import (
	"bytes"
	"container/list"
	"runtime"
)

type berEncoderFactory struct {
	bufPool *bufferPool
}

func newBerEncoderFactory(logger Logger) *berEncoderFactory {
	factory := new(berEncoderFactory)
	// Encoders are typically used and destroyed in short order, so we should only have a few active at any time. Each encoder may use quite a few small
	// temporary buffers during the encoding process though. Here we're setting things up for NumCpu * 2 encoders to be able to use 200 buffers each.
	factory.bufPool = newBufferPool(runtime.NumCPU()*2*200, 64, logger)
	return factory
}

type berEncoder struct {
	bufChain *list.List
	bufPool  *bufferPool
}

func (factory *berEncoderFactory) newBerEncoder() *berEncoder {
	encoder := new(berEncoder)
	encoder.bufChain = list.New()
	encoder.bufPool = factory.bufPool
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

type BerHeader struct {
	bytes.Buffer
}

func (encoder *berEncoder) newHeader(blockType snmpBlockType) *BerHeader {
	h := BerHeader{*encoder.append()}
	h.WriteByte(byte(blockType))
	return &h
}

func (h *BerHeader) setContentLength(contentLength int) (headerLength int, blockLength int) {
	if contentLength < 127 {
		h.WriteByte(byte(contentLength))
	} else {
		n := calculateLengthLen(contentLength)
		h.WriteByte(0x80 | n)
		for ; n > 0; n-- {
			h.WriteByte(byte(contentLength >> uint((n-1)*8)))
		}
	}
	headerLength = h.Len()
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

func encode2sComplementInt(buf *bytes.Buffer, val int64) {
	numBytesToWrite := calculate2sComplementIntLen(val)
	for i := numBytesToWrite; i > 0; i-- {
		buf.WriteByte(byte(val >> uint((i-1)*8)))
	}
	return
}

func calculate2sComplementIntLen(val int64) int {
	numBytes := 1
	for ; val > 127; val >>= 8 {
		numBytes++
	}
	for ; val < -128; val >>= 8 {
		numBytes++
	}
	return numBytes
}

func encodeBase128Int(buf *bytes.Buffer, val int64) int {
	if val == 0 {
		buf.WriteByte(0)
		return 1
	}
	numBytesToWrite := calculateBase128IntLen(val)
	for i := numBytesToWrite - 1; i >= 0; i-- {
		byteToWrite := byte(val>>uint(i*7)) & 0x7f
		if i != 0 {
			byteToWrite |= 0x80
		}
		buf.WriteByte(byteToWrite)
	}
	return int(numBytesToWrite)
}

func calculateBase128IntLen(val int64) int {
	numBytes := 0
	for i := val; i > 0; i >>= 7 {
		numBytes++
	}
	return numBytes
}
