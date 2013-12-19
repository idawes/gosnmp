package asn

import (
	"fmt"
	. "github.com/idawes/gosnmp/common"
)

type OctectString []byte

// encodeOctetString writes an octet string to the encoder. It returns the number of bytes written to the encoder
func (encoder *BerEncoder) encodeOctetString(val OctectString) int {
	h := encoder.newHeader(OCTET_STRING)
	buf := encoder.append()
	buf.Write(val)
	_, encodedLength := h.setContentLength(buf.Len())
	return encodedLength
}

func (decoder *BerDecoder) decodeOctetStringWithHeader() (OctectString, error) {
	startingPos := decoder.pos
	blockType, blockLength, err := decoder.decodeHeader()
	if err != nil {
		return nil, err
	}
	if blockType != OCTET_STRING {
		return nil, fmt.Errorf("Expecting type OCTET_STRING (0x%x), found 0x%x at pos %d", OCTET_STRING, blockType, startingPos)
	}
	return decoder.decodeOctetString(blockLength)
}

func (decoder *BerDecoder) decodeOctetString(numBytes int) (OctectString, error) {
	if numBytes > decoder.Len() {
		return nil, fmt.Errorf("Length %d for octet string exceeds available number of bytes %d at pos %d", numBytes, decoder.Len(), decoder.pos)
	}
	val := make(OctectString, numBytes)
	if numRead, err := decoder.Read(val); err != nil || numRead != numBytes {
		return nil, fmt.Errorf("Could only read %d of %d bytes for octet string at pos %d, err: %s", numRead, numBytes, decoder.pos, err)
	}
	decoder.pos += numBytes
	return val, nil
}
