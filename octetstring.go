package snmp_go

import (
	"fmt"
)

type OctectString []byte

// encodeOctetString writes an octet string to the encoder. It returns the number of bytes written to the encoder
func (encoder *berEncoder) encodeOctetString(val OctectString) int {
	h := encoder.newHeader(OCTET_STRING)
	buf := encoder.append()
	buf.Write(val)
	_, encodedLength := h.setContentLength(buf.Len())
	return encodedLength
}

func (decoder *berDecoder) decodeOctetStringWithHeader() (OctectString, error) {
	blockType, blockLength, err := decoder.decodeHeader()
	if err != nil {
		return nil, err
	}
	if blockType != OCTET_STRING {
		return nil, fmt.Errorf("Expecting type OCTET_STRING (0x%x), found 0x%x", OCTET_STRING, blockType)
	}
	return decoder.decodeOctetString(blockLength)
}

func (decoder *berDecoder) decodeOctetString(numBytes int) (OctectString, error) {
	if numBytes > decoder.Len() {
		return nil, fmt.Errorf("Length %d for octet string exceeds available number of bytes %d", numBytes, decoder.Len())
	}
	val := make(OctectString, numBytes)
	if numRead, err := decoder.Read(val); err != nil || numRead != numBytes {
		return nil, fmt.Errorf("Couldn't decode octet string of length %d. Number of bytes read from stream: %d, err: %s", numBytes, numRead, err)
	}
	return val, nil
}
