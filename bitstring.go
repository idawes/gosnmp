package snmp_go

import (
	"fmt"
)

// BitString is the structure to use when you want an ASN.1 BIT STRING type. A bit string is padded up to the nearest byte in memory
// and the number of valid bits is recorded. Padding bits will be zero
type BitString struct {
	bytes     []byte
	bitLength int
}

func NewBitString(numBits int) *BitString {
	bitString := new(BitString)
	numBytes := numBits / 8
	if numBits%8 != 0 {
		numBytes += 1
	}
	bitString.bytes = make([]byte, numBytes)
	bitString.bitLength = numBits
	return bitString
}

func (val *BitString) IsSet(bitIdx int) bool {
	if bitIdx < 0 || bitIdx > val.bitLength {
		return false
	}
	byteIdx := bitIdx / 8
	bitShift := 7 - uint(bitIdx%8)
	return int(val.bytes[byteIdx]>>bitShift)&1 == 1
}

func (val *BitString) Set(bitIdx int) error {
	if bitIdx < 0 || bitIdx > val.bitLength {
		return fmt.Errorf("Bit index %d is out of range. Max: %d", bitIdx, val.bitLength)
	}
	byteIdx := bitIdx / 8
	bitShift := 7 - uint(bitIdx%8)
	val.bytes[byteIdx] |= 1 << bitShift
	return nil
}

func (val *BitString) Clear(bitIdx int) error {
	if bitIdx < 0 || bitIdx > val.bitLength {
		return fmt.Errorf("Bit index %d is out of range. Max: %d", bitIdx, val.bitLength)
	}
	byteIdx := bitIdx / 8
	bitShift := 7 - uint(bitIdx%8)
	val.bytes[byteIdx] &^= (1 << bitShift)
	return nil
}

func (encoder *berEncoder) encodeBitString(val *BitString) int {
	h := encoder.newHeader(BIT_STRING)
	buf := encoder.append()
	numPaddingBits := byte((8 - val.bitLength%8) % 8)
	buf.WriteByte(numPaddingBits)
	buf.Write(val.bytes)
	_, encodedLength := h.setContentLength(buf.Len())
	return encodedLength
}

func (decoder *berDecoder) decodeBitString(numBytes int) (*BitString, error) {
	if numBytes < 1 {
		return nil, fmt.Errorf("Invalid length for bit string: %d", numBytes)
	}
	if numBytes > decoder.Len() {
		return nil, fmt.Errorf("Length %d for bitstring exceeds available number of bytes %d", numBytes, decoder.Len())
	}
	numPaddingBits, err := decoder.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("Couldn't read number of padding bits - err: %s", err)
	}
	if numPaddingBits < 0 || numPaddingBits > 7 {
		return nil, fmt.Errorf("Invalid number of padding bits: %d", numPaddingBits)
	}
	val := new(BitString)
	val.bytes = make([]byte, numBytes-1)
	numRead, err := decoder.Read(val.bytes)
	if err != nil || numRead != numBytes-1 {
		return nil, fmt.Errorf("Couldn't read bit string of length %d. Number of bytes read: %d", numBytes, numRead)
	}
	val.bitLength = ((numBytes - 1) * 8) - int(numPaddingBits)
	return val, nil
}
