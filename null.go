package gosnmp

import ()

func (encoder *berEncoder) encodeNull() (encodedLength int) {
	h := encoder.newHeader(snmpBlockType_NULL)
	_, encodedLength = h.setContentLength(0)
	return
}
