package asn

import (
	"fmt"
	. "github.com/idawes/gosnmp/common"
	"net"
)

// encodeIPv4Address writes an IPv4 address to the encoder. It returns the number of bytes written to the encoder
func (encoder *BerEncoder) encodeIPv4Address(addr net.IP) (int, error) {
	ipv4Addr := addr.To4()
	if ipv4Addr == nil {
		return 0, fmt.Errorf("IP Address %s is not a valid v4 address", addr.String())
	}
	h := encoder.newHeader(IP_ADDRESS)
	buf := encoder.append()
	buf.Write(addr)
	_, encodedLength := h.setContentLength(buf.Len())
	return encodedLength, nil
}

func (decoder *BerDecoder) decodeIPv4AddressWithHeader() (net.IP, error) {
	startingPos := decoder.pos
	blockType, blockLength, err := decoder.decodeHeader()
	if err != nil {
		return net.IPv4zero, err
	}
	if blockType != IP_ADDRESS {
		return net.IPv4zero, fmt.Errorf("Expecting type IP_ADDRESS (0x%x), found 0x%x at pos %d", IP_ADDRESS, blockType, startingPos)
	}
	return decoder.decodeIPv4Address(blockLength)
}

func (decoder *BerDecoder) decodeIPv4Address(numBytes int) (net.IP, error) {
	if numBytes > decoder.Len() {
		return net.IPv4zero, fmt.Errorf("Length %d for IPv4 address exceeds available number of bytes %d at pos %d", numBytes, decoder.Len(), decoder.pos)
	}
	if numBytes != 4 {
		return net.IPv4zero, fmt.Errorf("Length %d for IPv4 address is incorrect at pos %d", numBytes, decoder.pos)
	}
	decoder.pos += 4
	addrBytes := make([]byte, 4)
	if numRead, err := decoder.Read(addrBytes); numRead != 4 || err != nil {
		return net.IPv4zero, fmt.Errorf("Read %d bytes instead of 4 from decoder at pos %d while decoding IPv4 address. err: %s", numRead, decoder.pos, err)
	}
	return net.IPv4(addrBytes[0], addrBytes[1], addrBytes[2], addrBytes[3]), nil
}
