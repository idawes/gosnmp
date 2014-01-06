package gosnmp

import (
	"fmt"
	"math"
)

// An ObjectIdentifier represents an ASN.1 OBJECT IDENTIFIER.
type ObjectIdentifier []uint32

// Equal returns true iff a and b represent the same identifier.
func (a ObjectIdentifier) Equal(b ObjectIdentifier) bool {
	if len(a) != len(b) {
		return false
	}
	for i, a_i := range a {
		if a_i != b[i] {
			return false
		}
	}
	return true
}

// Compare returns -1 if a comes before b in the oid tree, 0 if a and b represent exactly the same node in the oid tree and 1 if a comes after b in the oid tree.
func (a ObjectIdentifier) Compare(b ObjectIdentifier) int {
	bLen := len(b)
	for i, a_i := range a {
		if i == bLen {
			// we ran out of elements in oid b first, and all elements to this point were equal. Therefore oid b is less specific than a, and comes first in the tree
			return 1
		}
		diff := int64(a_i) - int64(b[i])
		if diff != 0 {
			return int(diff)
		}
	}
	if len(a) == bLen {
		// all elements were equal
		return 0
	}
	// we ran out of elements in oid a first, and all elements to this point were equal, Therefore oid a is less specific than b, and comes first in the tree.
	return -1
}

func (a ObjectIdentifier) MatchLength(b ObjectIdentifier) int {
	bLen := len(b)
	for i, a_i := range a {
		if i == bLen {
			// we ran out of elements in oid b first, and all elements to this point were equal.
			return bLen
		}
		if a_i != b[i] {
			return i
		}
	}
	return len(a)
}

// encodeObjectIdentifier writes an object identifier to the encoder. It returns the number of bytes written to the encoder
func (encoder *berEncoder) encodeObjectIdentifier(oid ObjectIdentifier) (int, error) {
	if len(oid) < 2 || oid[0] > 6 || oid[1] >= 40 {
		return 0, fmt.Errorf("Invalid oid: %v", oid)
	}
	h := encoder.newHeader(snmpBlockType_OBJECT_IDENTIFIER)
	buf := encoder.append()
	buf.WriteByte(byte(oid[0]*40 + oid[1])) // first byte holds the first two identifiers in the oid
	for i := 2; i < len(oid); i++ {         // remaining oid identifiers are marshalled as base 128 integers
		encodeBase128Int(buf, int64(oid[i]))
	}
	_, encodedLength := h.setContentLength(buf.Len())
	return encodedLength, nil
}

func (decoder *berDecoder) decodeObjectIdentifierWithHeader() (ObjectIdentifier, error) {
	startingPos := decoder.pos
	blockType, blockLength, err := decoder.decodeHeader()
	if err != nil {
		return nil, err
	}
	if blockType != snmpBlockType_OBJECT_IDENTIFIER {
		return nil, fmt.Errorf("Expecting type snmpBlockType_OBJECT_IDENTIFIER (0x%x), found 0x%x at pos %d", snmpBlockType_OBJECT_IDENTIFIER, blockType, startingPos)
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

func (decoder *berDecoder) decodeBase128Int() (int64, error) {
	var val int64
	numBytesRead := 0
	for ; ; numBytesRead++ {
		if numBytesRead == 4 {
			return 0, fmt.Errorf("Base 128 integer too large at pos %d", decoder.pos)
		}
		val <<= 7
		b, err := decoder.ReadByte()
		if err != nil {
			return 0, fmt.Errorf("Couldn't read byte %d of base 128 integer at pos %d", numBytesRead+1, decoder.pos)
		}
		val |= int64(b & 0x7f)
		if b&0x80 == 0 {
			break
		}
	}
	decoder.pos += numBytesRead + 1
	return val, nil
}
