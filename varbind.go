package snmp_go

import (
	"bytes"
)

type Varbind interface {
	Marshal(bufChain *bufferChain) (marshalledLen int)
}

type baseVarbind struct {
	oid           []int32
	varbindHeader *bytes.Buffer
	oidHeader     *bytes.Buffer
	oidBody       *bytes.Buffer
	valHeader     *bytes.Buffer
	valBody       *bytes.Buffer
}

func (vb *baseVarbind) prepareMarshallingBuffers(bufChain *bufferChain) {
	vb.varbindHeader = bufChain.addBufToTail()
	vb.oidHeader = bufChain.addBufToTail()
	vb.oidBody = bufChain.addBufToTail()
	vb.valHeader = bufChain.addBufToTail()
	vb.valBody = bufChain.addBufToTail()
}

type IntegerVarbind struct { // type 0x02
	baseVarbind
	val int32
}

func (vb *IntegerVarbind) Marshal(bufChain *bufferChain) (marshalledLen int) {
	vb.prepareMarshallingBuffers(bufChain)
	len := marshalInteger(vb.valHeader, vb.valBody, int64(vb.val))
	len += marshalObjectIdentifier(vb.oidHeader, vb.oidBody, vb.oid)
	return len + marshalTypeAndLength(vb.varbindHeader, SEQUENCE, len)
}

type BitStringVarbind struct { // type 0x03
	baseVarbind
	val []byte
}

type OctetStringVarbind struct { // type 0x04
	baseVarbind
	val []byte
}

func (vb *OctetStringVarbind) Marshal(bufChain *bufferChain) (marshalledLen int) {
	vb.prepareMarshallingBuffers(bufChain)
	len := marshalOctetString(vb.valHeader, vb.valBody, vb.val)
	len += marshalObjectIdentifier(vb.oidHeader, vb.oidBody, vb.oid)
	return len + marshalTypeAndLength(vb.varbindHeader, SEQUENCE, len)
}

type NullVarbind struct { // type 0x05
	baseVarbind
}

func NewNullVarbind(oid []int32) *NullVarbind {
	vb := new(NullVarbind)
	vb.oid = oid
	return vb
}

func (vb *NullVarbind) Marshal(bufChain *bufferChain) (marshalledLen int) {
	vb.prepareMarshallingBuffers(bufChain)
	len := marshalTypeAndLength(vb.valHeader, NULL, 0)
	len += marshalObjectIdentifier(vb.oidHeader, vb.oidBody, vb.oid)
	return len + marshalTypeAndLength(vb.varbindHeader, SEQUENCE, len)
}

type ObjectIdentifierVarbind struct { // type 0x06
	baseVarbind
	val []int32
}

func (vb *ObjectIdentifierVarbind) Marshal(bufChain *bufferChain) (marshalledLen int) {
	vb.prepareMarshallingBuffers(bufChain)
	len := marshalObjectIdentifier(vb.valHeader, vb.valBody, vb.val)
	len += marshalObjectIdentifier(vb.oidHeader, vb.oidBody, vb.oid)
	return len + marshalTypeAndLength(vb.varbindHeader, SEQUENCE, len)
}

type IpAddressVarbind struct { // type 0x40
	baseVarbind
	val [4]byte
}

type Counter32Varbind struct { // type 0x41
	baseVarbind
	val uint32
}

type Gauge32Varbind struct { // type 0x42
	baseVarbind
	val uint32
}

type TimeTicksVarbind struct { // type 0x43
	baseVarbind
	val uint32
}

type OpaqueVarbind struct { // type 0x44
	baseVarbind
	val []byte
}

type NsapAddressVarbind struct { // type 0x45
	baseVarbind
	val [6]byte
}

type Counter64Varbind struct { // type 0x46
	baseVarbind
	val uint64
}

type Uinteger32Varbind struct { // type 0x47
	baseVarbind
	val uint32
}
