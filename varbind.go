package snmp_go

import (
	. "github.com/idawes/ber_go"
)

type Varbind interface {
	encode(encoder *BerEncoder) (marshalledLen int)
}

type baseVarbind struct {
	oid []uint32
}

type IntegerVarbind struct { // type 0x02
	baseVarbind
	val int32
}

func (vb *IntegerVarbind) encode(encoder *BerEncoder) (marshalledLen int) {
	header := encoder.NewHeader(SEQUENCE)
	len := encoder.EncodeObjectIdentifier(vb.oid)
	len += encoder.EncodeInteger(int64(vb.val))
	_, marshalledLen = header.SetContentLength(len)
	return
}

type BitStringVarbind struct { // type 0x03
	baseVarbind
	val []byte
}

type OctetStringVarbind struct { // type 0x04
	baseVarbind
	val []byte
}

func (vb *OctetStringVarbind) encode(encoder *BerEncoder) (marshalledLen int) {
	header := encoder.NewHeader(SEQUENCE)
	len := encoder.EncodeObjectIdentifier(vb.oid)
	len += encoder.EncodeOctetString(vb.val)
	_, marshalledLen = header.SetContentLength(len)
	return
}

type NullVarbind struct { // type 0x05
	baseVarbind
}

func NewNullVarbind(oid []uint32) *NullVarbind {
	vb := new(NullVarbind)
	vb.oid = oid
	return vb
}

func (vb *NullVarbind) encode(encoder *BerEncoder) (marshalledLen int) {
	header := encoder.NewHeader(SEQUENCE)
	len := encoder.EncodeObjectIdentifier(vb.oid)
	_, marshalledLen = header.SetContentLength(len)
	return
}

type ObjectIdentifierVarbind struct { // type 0x06
	baseVarbind
	val []uint32
}

func (vb *ObjectIdentifierVarbind) encode(encoder *BerEncoder) (marshalledLen int) {
	header := encoder.NewHeader(SEQUENCE)
	len := encoder.EncodeObjectIdentifier(vb.oid)
	len += encoder.EncodeObjectIdentifier(vb.val)
	_, marshalledLen = header.SetContentLength(len)
	return
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
