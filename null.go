package gosnmp

import ()

func (encoder *berEncoder) encodeNull(nullType snmpBlockType) (encodedLength int) {
	h := encoder.newHeader(nullType)
	_, encodedLength = h.setContentLength(0)
	return
}
