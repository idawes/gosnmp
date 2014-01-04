package gosnmp

import (
	"bytes"
	"fmt"
)

type berDecoder struct {
	*bytes.Buffer
	pos int
}

func newberDecoder(msg []byte) *berDecoder {
	decoder := berDecoder{bytes.NewBuffer(msg), 0}
	return &decoder
}

// decodeHeader pulls an ASN.1 block header from the decoder. It returns the decoded type and length of the block.
func (decoder *berDecoder) decodeHeader() (snmpBlockType, int, error) {
	blockType, err := decoder.ReadByte()
	if err != nil {
		return 0, 0, fmt.Errorf("Couldn't read byte at pos %d, err: %s", decoder.pos, err)
	}
	decoder.pos++
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
		return 0, fmt.Errorf("Couldn't read byte at pos %d, err: %s", decoder.pos, err)
	}
	decoder.pos++
	if firstByte < 127 {
		length = int(firstByte)
		return length, nil
	}
	for numBytes := firstByte; numBytes > 0; numBytes-- {
		temp, err := decoder.ReadByte()
		if err != nil {
			return 0, fmt.Errorf("Couldn't read byte at pos %d, err: %s", decoder.pos, err)
		}
		decoder.pos++
		length <<= 8
		length += int(temp)
	}
	if length < 0 {
		return 0, fmt.Errorf("Decoding length field found negative value: %d at pos %d", length, decoder.pos)
	}
	return length, nil
}

// decodeValue pulls a single basic value TLV from the decoder. It returns the value's type and the value as a generic.
func (decoder *berDecoder) decodeValue() (snmpBlockType, interface{}, error) {
	valueType, valueLength, err := decoder.decodeHeader()
	if err != nil {
		return 0, nil, fmt.Errorf("Unable to decode value header at pos %d - err: %s", decoder.pos, err)
	}
	var value interface{}
	switch valueType {
	case snmpBlockType_INTEGER:
		value, err = decoder.decodeInteger(valueLength)
	case snmpBlockType_BIT_STRING:
		value, err = decoder.decodeBitString(valueLength)
	case snmpBlockType_OCTET_STRING:
		value, err = decoder.decodeOctetString(valueLength)
	case snmpBlockType_NULL, snmpBlockType_NO_SUCH_OBJECT, snmpBlockType_NO_SUCH_INSTANCE, snmpBlockType_END_OF_MIB_VIEW:
		value = nil
	case snmpBlockType_OBJECT_IDENTIFIER:
		value, err = decoder.decodeObjectIdentifier(valueLength)
	case snmpBlockType_SEQUENCE:
		return 0, nil, fmt.Errorf("Unexpected value type snmpBlockType_SEQUENCE 0x%x at pos %d", valueType, decoder.pos)
	case snmpBlockType_IP_ADDRESS:
		value, err = decoder.decodeIPv4Address(valueLength)
	case snmpBlockType_COUNTER_32:
		// value, err = decoder.decodeCounter32(valueLength)
	case snmpBlockType_GAUGE_32:
		// value, err = decoder.decodeGauge32(valueLength)
	case snmpBlockType_TIME_TICKS:
		// value, err = decoder.decodeTimeTicks(valueLength)
	case snmpBlockType_OPAQUE:
		// value, err = decoder.decodeOpaque(valueLength)
	case snmpBlockType_COUNTER_64:
		// value, err = decoder.decodeCounter64(valueLength)
	case snmpBlockType_UINT_32:
		// value, err = decoder.decodeUint32(valueLength)
	default:
		return 0, nil, fmt.Errorf("Unknown value type 0x%x", valueType)
	}
	return valueType, value, nil
}
