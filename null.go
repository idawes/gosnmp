package snmp_go

func (encoder *berEncoder) encodeNull() (encodedLength int) {
	h := encoder.newHeader(NULL)
	_, encodedLength = h.setContentLength(0)
	return
}
