package gosnmp

import (
	"fmt"
	"math"
)

// An ObjectIdentifier represents an ASN.1 OBJECT IDENTIFIER.
type ObjectIdentifier []uint32

// Equal returns true iff oi and other represent the same identifier.
func (oid ObjectIdentifier) Equal(other ObjectIdentifier) bool {
	if len(oid) != len(other) {
		return false
	}
	for i := 0; i < len(oid); i++ {
		if oid[i] != other[i] {
			return false
		}
	}
	return true
}

// encodeObjectIdentifier writes an object identifier to the encoder. It returns the number of bytes written to the encoder
func (encoder *berEncoder) encodeObjectIdentifier(oid ObjectIdentifier) int {
	if len(oid) < 2 || oid[0] > 6 || oid[1] >= 40 {
		panic(fmt.Sprintf("Invalid oid: %v", oid))
	}
	h := encoder.newHeader(OBJECT_IDENTIFIER)
	buf := encoder.append()
	buf.WriteByte(byte(oid[0]*40 + oid[1])) // first byte holds the first two identifiers in the oid
	for i := 2; i < len(oid); i++ {         // remaining oid identifiers are marshalled as base 128 integers
		encodeBase128Int(buf, int64(oid[i]))
	}
	_, encodedLength := h.setContentLength(buf.Len())
	return encodedLength
}

func (decoder *berDecoder) decodeObjectIdentifierWithHeader() (ObjectIdentifier, error) {
	startingPos := decoder.pos
	blockType, blockLength, err := decoder.decodeHeader()
	if err != nil {
		return nil, err
	}
	if blockType != OBJECT_IDENTIFIER {
		return nil, fmt.Errorf("Expecting type OBJECT_IDENTIFIER (0x%x), found 0x%x at pos %d", OBJECT_IDENTIFIER, blockType, startingPos)
	}
	return decoder.decodeObjectIdentifier(blockLength)
}

func (decoder *berDecoder) decodeObjectIdentifier(numBytes int) (ObjectIdentifier, error) {
	if numBytes > decoder.Len() {
		return nil, fmt.Errorf("Length %d for object identifier exceeds available number of bytes %d at pos %d", numBytes, decoder.Len(), decoder.pos)
	}

	// In the worst case, we get two elements from the first byte (which is encoded differently) and then every varint is a single byte long
	oid := make(ObjectIdentifier, numBytes+1)

	startingPos := decoder.pos
	// The first byte is 40*value1 + value2
	firstByte, err := decoder.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("Couldn't read first byte of object identifier at pos %d, err: %s", err, decoder.pos)
	}
	decoder.pos++
	oid[0] = uint32(firstByte) / 40
	oid[1] = uint32(firstByte) % 40
	numVals := 2
	for ; ; numVals++ {
		fmt.Println(numVals, decoder.pos, startingPos, numBytes)
		identifierPos := decoder.pos
		tval, err := decoder.decodeBase128Int()
		if err != nil {
			return nil, fmt.Errorf("Couldn't decode identifier at level %d, err: %s", numVals, err)
		}
		if tval < 0 || tval > math.MaxUint32 {
			return nil, fmt.Errorf("Invalid identifier %d at level %d pos %d", tval, numVals, identifierPos)
		}
		oid[numVals] = uint32(tval)
		if decoder.pos-startingPos == numBytes {
			break
		}
		if decoder.pos-startingPos > numBytes {
			return nil, fmt.Errorf("Decoding object identifier at pos %d consumed too many bytes: %d vs expected %d", startingPos, decoder.pos-startingPos, numBytes)
		}
	}
	oid = oid[0 : numVals+1]
	return oid, nil
}
