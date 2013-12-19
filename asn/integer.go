package asn

import (
	"bytes"
	"fmt"
	. "github.com/idawes/gosnmp/common"
)

// IntegerVarbind stuff
type IntegerVarbind struct { // type 0x02
	baseVarbind
	val int32
}

func NewIntegerVarbind(oid ObjectIdentifier, val int32) *IntegerVarbind {
	vb := new(IntegerVarbind)
	vb.oid = oid
	vb.val = val
	return vb
}

func (vb *IntegerVarbind) encodeValue(encoder *BerEncoder) (int, error) {
	return encoder.encodeInteger(int64(vb.val)), nil
}

func (vb *IntegerVarbind) decodeValue(decoder *BerDecoder, valueLength int) (err error) {
	vb.val, err = decoder.decodeInt32(valueLength)
	return
}

////////////////////////////////////////////////////////////////////////////
// Integer BER encode
func (encoder *BerEncoder) encodeInteger(val int64) (encodedLength int) {
	h := encoder.newHeader(INTEGER)
	buf := encoder.append()
	encode2sComplementInt(buf, val)
	_, encodedLength = h.setContentLength(buf.Len())
	return
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

////////////////////////////////////////////////////////////////////////////
// Integer BER decode
func (decoder *BerDecoder) decodeIntegerWithHeader() (int64, error) {
	blockLength, err := decoder.decodeIntegerHeader()
	if err != nil {
		return 0, err
	}
	return decoder.decodeInteger(blockLength)
}

func (decoder *BerDecoder) decodeIntegerHeader() (int, error) {
	startingPos := decoder.pos
	blockType, blockLength, err := decoder.decodeHeader()
	if err != nil {
		return 0, err
	}
	if blockType != INTEGER {
		return 0, fmt.Errorf("Expecting type INTEGER (0x%x) at pos %d, found 0x%x", INTEGER, startingPos, blockType)
	}
	return blockLength, nil
}

func (decoder *BerDecoder) decodeInteger(blockLength int) (int64, error) {
	return decoder.decode2sComplementInt(blockLength)
}

func (decoder *BerDecoder) decode2sComplementInt(numBytes int) (int64, error) {
	var val int64
	for i := 0; i < numBytes; i++ {
		temp, err := decoder.ReadByte()
		if err != nil {
			return 0, fmt.Errorf("Couldn't read byte at pos %d, err: %s", decoder.pos, err)
		}
		decoder.pos++
		val <<= 8
		val |= int64(temp)
	}

	// Shift up and down in order to sign extend the result.
	val <<= 64 - uint8(numBytes)*8
	val >>= 64 - uint8(numBytes)*8
	return val, nil
}

func (decoder *BerDecoder) decodeInt32WithHeader() (int32, error) {
	blockLength, err := decoder.decodeIntegerHeader()
	if err != nil {
		return 0, err
	}
	return decoder.decodeInt32(blockLength)
}

func (decoder *BerDecoder) decodeInt32(valueLength int) (int32, error) {
	startingPos := decoder.pos
	rawVal, err := decoder.decodeInteger(valueLength)
	if err != nil {
		return 0, err
	}
	val := int32(rawVal)
	if int64(val) != rawVal {
		return 0, fmt.Errorf("Value %d out of int32 range at pos %d", rawVal, startingPos)
	}
	return val, nil
}

func (decoder *BerDecoder) decodeUint32WithHeader() (uint32, error) {
	blockLength, err := decoder.decodeIntegerHeader()
	if err != nil {
		return 0, err
	}
	return decoder.decodeUint32(blockLength)
}

func (decoder *BerDecoder) decodeUint32(valueLength int) (uint32, error) {
	startingPos := decoder.pos
	rawVal, err := decoder.decodeInteger(valueLength)
	if err != nil {
		return 0, err
	}
	val := uint32(rawVal)
	if int64(val) != rawVal {
		return 0, fmt.Errorf("Value %d out of uint32 range at pos %d", rawVal, startingPos)
	}
	return val, nil
}
