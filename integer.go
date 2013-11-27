package gosnmp

import (
	"fmt"
)

func (encoder *berEncoder) encodeInteger(val int64) (encodedLength int) {
	h := encoder.newHeader(INTEGER)
	buf := encoder.append()
	encode2sComplementInt(buf, val)
	_, encodedLength = h.setContentLength(buf.Len())
	return
}

func (decoder *berDecoder) decodeIntegerWithHeader() (val int64, err error) {
	startingPos := decoder.pos
	blockType, blockLength, err := decoder.decodeHeader()
	if err != nil {
		return 0, err
	}
	if blockType != INTEGER {
		return 0, fmt.Errorf("Expecting type INTEGER (0x%x) at pos %d, found 0x%x", startingPos, INTEGER, blockType)
	}
	return decoder.decodeInteger(blockLength)
}

func (decoder *berDecoder) decodeInteger(blockLength int) (val int64, err error) {
	val, err = decoder.decode2sComplementInt(blockLength)
	return
}

func (decoder *berDecoder) decodeInt32WithHeader() (val int32, err error) {
	startingPos := decoder.pos
	blockType, blockLength, err := decoder.decodeHeader()
	if err != nil {
		return 0, err
	}
	if blockType != INTEGER {
		return 0, fmt.Errorf("Expecting type INTEGER (0x%x) at pos %d, found 0x%x", startingPos, INTEGER, blockType)
	}
	return decoder.decodeInt32(blockLength)
}

func (decoder *berDecoder) decodeInt32(valueLength int) (val int32, err error) {
	startingPos := decoder.pos
	rawVal, err := decoder.decodeInteger(valueLength)
	if err != nil {
		return 0, err
	}
	val = int32(rawVal)
	if int64(val) != rawVal {
		return 0, fmt.Errorf("Value %d out of int32 range at pos %d", rawVal, startingPos)
	}
	return
}

func (decoder *berDecoder) decodeUint32WithHeader() (val uint32, err error) {
	startingPos := decoder.pos
	blockType, blockLength, err := decoder.decodeHeader()
	if err != nil {
		return 0, err
	}
	if blockType != INTEGER {
		return 0, fmt.Errorf("Expecting type INTEGER (0x%x) at pos %d, found 0x%x", startingPos, INTEGER, blockType)
	}
	return decoder.decodeUint32(blockLength)
}

func (decoder *berDecoder) decodeUint32(valueLength int) (val uint32, err error) {
	startingPos := decoder.pos
	rawVal, err := decoder.decodeInteger(valueLength)
	if err != nil {
		return 0, err
	}
	val = uint32(rawVal)
	if int64(val) != rawVal {
		return 0, fmt.Errorf("Value %d out of uint32 range at pos %d", rawVal, startingPos)
	}
	return
}
