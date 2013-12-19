package asn

import (
	. "github.com/idawes/gosnmp/common"
)

func (encoder *BerEncoder) encodeNull() (encodedLength int) {
	h := encoder.newHeader(NULL)
	_, encodedLength = h.setContentLength(0)
	return
}
