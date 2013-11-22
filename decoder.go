package gosnmp

import (
	"bytes"
	"fmt"
)

type berDecoder struct {
	*bytes.Buffer
}

func newBerDecoder(msg []byte) *berDecoder {
	decoder := berDecoder{bytes.NewBuffer(msg)}
	return &decoder
}

// decodeHeader pulls an ASN.1 block header from the decoder. It returns the decoded type and length of the block.
func (decoder *berDecoder) decodeHeader() (snmpBlockType, int, error) {
	blockType, err := decoder.ReadByte()
	if err != nil {
		return 0, 0, err
	}
	blockLength, err := decoder.decodeLength()
	if err != nil {
		return 0, 0, err
	}
	if blockLength > decoder.Len() {
		return 0, 0, fmt.Errorf("Length %d for block exceeds remaining message length %d", blockLength, decoder.Len())
	}
	return snmpBlockType(blockType), blockLength, nil
}

// Note: returned length will never be negative.
func (decoder *berDecoder) decodeLength() (int, error) {
	var length int
	firstByte, err := decoder.ReadByte()
	if err != nil {
		return 0, err
	}
	if firstByte < 127 {
		length = int(firstByte)
		return length, nil
	}
	for numBytes := firstByte; numBytes > 0; numBytes-- {
		temp, err := decoder.ReadByte()
		if err != nil {
			return 0, err
		}
		length <<= 8
		length += int(temp)
	}
	if length < 0 {
		return 0, fmt.Errorf("Decoding length field found negative value: %d", length)
	}
	return length, nil
}

// decodeValue pulls a single basic value TLV from the decoder. It returns the value's type and the value as a generic.
func (decoder *berDecoder) decodeValue() (snmpBlockType, interface{}, error) {
	valueType, valueLength, err := decoder.decodeHeader()
	if err != nil {
		return 0, nil, fmt.Errorf("Unable to decode value header - err: %s", err)
	}
	var value interface{}
	switch valueType {
	case INTEGER:
		value, err = decoder.decodeInteger(valueLength)
	case BIT_STRING:
		value, err = decoder.decodeBitString(valueLength)
	case OCTET_STRING:
		value, err = decoder.decodeOctetString(valueLength)
	case NULL:
		value = nil
	case OBJECT_IDENTIFIER:
		value, err = decoder.decodeObjectIdentifier(valueLength)
	case SEQUENCE:
		return 0, nil, fmt.Errorf("Unexpected value type SEQUENCE 0x%x", valueType)
	case IP_ADDRESS:
		// value, err = decoder.decodeIpAddress(valueLength)
	case COUNTER_32:
		// value, err = decoder.decodeCounter32(valueLength)
	case GAUGE_32:
		// value, err = decoder.decodeGauge32(valueLength)
	case TIME_TICKS:
		// value, err = decoder.decodeTimeTicks(valueLength)
	case OPAQUE:
		// value, err = decoder.decodeOpaque(valueLength)
	case COUNTER_64:
		// value, err = decoder.decodeCounter64(valueLength)
	case UINT_32:
		// value, err = decoder.decodeUint32(valueLength)
	default:
		return 0, nil, fmt.Errorf("Unknown value type 0x%x", valueType)
	}
	return valueType, value, nil
}

func (decoder *berDecoder) decode2sComplementInt(numBytes int) (int64, error) {
	var val int64
	for i := 0; i < numBytes; i++ {
		temp, err := decoder.ReadByte()
		if err != nil {
			return 0, err
		}
		val <<= 8
		val |= int64(temp)
	}

	// Shift up and down in order to sign extend the result.
	val <<= 64 - uint8(numBytes)*8
	val >>= 64 - uint8(numBytes)*8
	return val, nil
}

func (decoder *berDecoder) decodeBase128Int() (int64, error) {
	var val int64
	for numBytesRead := 0; ; numBytesRead++ {
		if numBytesRead == 4 {
			return 0, fmt.Errorf("Base 128 integer too large")
		}
		val <<= 7
		b, err := decoder.ReadByte()
		if err != nil {
			return 0, fmt.Errorf("Couldn't read byte %d of base 128 integer", numBytesRead+1)
		}
		val |= int64(b & 0x7f)
		if b&0x80 == 0 {
			return val, nil
		}
	}
}
