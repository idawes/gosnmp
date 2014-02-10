package gosnmp

import (
	"fmt"
	"net"
)

type Varbind interface {
	//encodeValue returns the number of bytes written to the encoder
	encodeValue(encoder *berEncoder) (int, error)
	GetOid() ObjectIdentifier
	setOid(oid ObjectIdentifier)
}

func (encoder *berEncoder) encodeVarbind(vb Varbind) (int, error) {
	header := encoder.newHeader(snmpBlockType_SEQUENCE)
	oidLen, err := encoder.encodeObjectIdentifier(vb.GetOid())
	if err != nil {
		return 0, err
	}
	valLen, err := vb.encodeValue(encoder)
	if err != nil {
		return 0, err
	}
	_, marshalledLen := header.setContentLength(oidLen + valLen)
	return marshalledLen, nil
}

type baseVarbind struct {
	oid ObjectIdentifier
}

func (vb *baseVarbind) GetOid() ObjectIdentifier {
	return vb.oid
}

func (vb *baseVarbind) setOid(oid ObjectIdentifier) {
	vb.oid = oid
}

type OctetStringVarbind struct { // type 0x04
	baseVarbind
	Value []byte
}

func NewStringVarbind(oid ObjectIdentifier, val string) *OctetStringVarbind {
	return NewOctetStringVarbind(oid, []byte(val))
}

func NewOctetStringVarbind(oid ObjectIdentifier, val []byte) *OctetStringVarbind {
	vb := new(OctetStringVarbind)
	vb.oid = oid
	vb.Value = val
	return vb
}

func (vb *OctetStringVarbind) encodeValue(encoder *berEncoder) (int, error) {
	return encoder.encodeOctetString(vb.Value), nil
}

type NullVarbind struct { // type 0x05
	baseVarbind
}

func NewNullVarbind(oid ObjectIdentifier) *NullVarbind {
	vb := new(NullVarbind)
	vb.oid = oid
	return vb
}

func (vb *NullVarbind) encodeValue(encoder *berEncoder) (int, error) {
	return encoder.encodeNull(snmpBlockType_NULL), nil
}

type ObjectIdentifierVarbind struct { // type 0x06
	baseVarbind
	Value ObjectIdentifier
}

func NewObjectIdentifierVarbind(oid ObjectIdentifier, val ObjectIdentifier) *ObjectIdentifierVarbind {
	vb := new(ObjectIdentifierVarbind)
	vb.oid = oid
	vb.Value = val
	return vb
}

func (vb *ObjectIdentifierVarbind) encodeValue(encoder *berEncoder) (int, error) {
	return encoder.encodeObjectIdentifier(vb.Value)
}

type IPv4AddressVarbind struct { // type 0x40
	baseVarbind
	Value net.IP
}

func NewIPv4AddressVarbind(oid ObjectIdentifier, val net.IP) *IPv4AddressVarbind {
	vb := new(IPv4AddressVarbind)
	vb.oid = oid
	vb.Value = val
	return vb
}

func (vb *IPv4AddressVarbind) encodeValue(encoder *berEncoder) (int, error) {
	return encoder.encodeIPv4Address(vb.Value)
}

type Counter32Varbind struct { // type 0x41
	baseVarbind
	Value uint32
}

func NewCounter32Varbind(oid ObjectIdentifier) *Counter32Varbind {
	vb := new(Counter32Varbind)
	vb.oid = oid
	return vb
}

type Gauge32Varbind struct { // type 0x42
	baseVarbind
	Value uint32
}

func NewGauge32Varbind(oid ObjectIdentifier) *Gauge32Varbind {
	vb := new(Gauge32Varbind)
	vb.oid = oid
	return vb
}

type TimeTicksVarbind struct { // type 0x43
	baseVarbind
	Value uint32
}

func NewTimeTicksVarbind(oid ObjectIdentifier) *TimeTicksVarbind {
	vb := new(TimeTicksVarbind)
	vb.oid = oid
	return vb
}

type OpaqueVarbind struct { // type 0x44
	baseVarbind
	Value []byte
}

func NewOpaqueVarbind(oid ObjectIdentifier) *OpaqueVarbind {
	vb := new(OpaqueVarbind)
	vb.oid = oid
	return vb
}

type NsapAddressVarbind struct { // type 0x45
	baseVarbind
	Value [6]byte
}

func NewNsapAddressVarbind(oid ObjectIdentifier) *NsapAddressVarbind {
	vb := new(NsapAddressVarbind)
	vb.oid = oid
	return vb
}

type Counter64Varbind struct { // type 0x46
	baseVarbind
	Value uint64
}

func NewCounter64Varbind(oid ObjectIdentifier) *Counter64Varbind {
	vb := new(Counter64Varbind)
	vb.oid = oid
	return vb
}

type Uint32Varbind struct { // type 0x47
	baseVarbind
	val uint32
}

func NewUint32Varbind(oid ObjectIdentifier) *Uint32Varbind {
	vb := new(Uint32Varbind)
	vb.oid = oid
	return vb
}

type NoSuchObjectVarbind struct { // type 0x80
	baseVarbind
}

func NewNoSuchObjectVarbind(oid ObjectIdentifier) *NoSuchObjectVarbind {
	vb := new(NoSuchObjectVarbind)
	vb.oid = oid
	return vb
}

func (vb *NoSuchObjectVarbind) encodeValue(encoder *berEncoder) (int, error) {
	return encoder.encodeNull(snmpBlockType_NO_SUCH_OBJECT), nil
}

type NoSuchInstanceVarbind struct { // type 0x81
	baseVarbind
}

func NewNoSuchInstanceVarbindVarbind(oid ObjectIdentifier) *NoSuchInstanceVarbind {
	vb := new(NoSuchInstanceVarbind)
	vb.oid = oid
	return vb
}

func (vb *NoSuchInstanceVarbind) encodeValue(encoder *berEncoder) (int, error) {
	return encoder.encodeNull(snmpBlockType_NO_SUCH_INSTANCE), nil
}

type EndOfMibViewVarbind struct { // type 0x82
	baseVarbind
}

func NewEndOfMibViewVarbind(oid ObjectIdentifier) *EndOfMibViewVarbind {
	vb := new(EndOfMibViewVarbind)
	vb.oid = oid
	return vb
}

func (vb *EndOfMibViewVarbind) encodeValue(encoder *berEncoder) (int, error) {
	return encoder.encodeNull(snmpBlockType_END_OF_MIB_VIEW), nil
}

func decodeVarbind(decoder *berDecoder) (varbind Varbind, err error) {
	varbindHeaderType, varbindLength, err := decoder.decodeHeader()
	if err != nil {
		return nil, fmt.Errorf("Unable to decode varbind header - err: %s", err)
	}
	startDecoderLen := decoder.Len()
	if varbindHeaderType != snmpBlockType_SEQUENCE {
		return nil, fmt.Errorf("Invalid varbind header type 0x%x - not 0x%x", varbindHeaderType, snmpBlockType_SEQUENCE)
	}
	oid, err := decoder.decodeObjectIdentifierWithHeader()
	if err != nil {
		return nil, fmt.Errorf("Failed to decode object identifier - err: %s", err)
	}
	valueType, value, err := decoder.decodeValue()
	if err != nil {
		return nil, fmt.Errorf("Unable to decode value header - err: %s", err)
	}
	switch valueType {
	case snmpBlockType_INTEGER:
		varbind = NewIntegerVarbind(oid, value.(int32))
	case snmpBlockType_BIT_STRING:
		varbind = NewBitStringVarbind(oid, value.(*BitString))
	case snmpBlockType_OCTET_STRING:
		varbind = NewOctetStringVarbind(oid, value.(OctectString))
	case snmpBlockType_NULL:
		varbind = NewNullVarbind(oid)
	case snmpBlockType_OBJECT_IDENTIFIER:
		varbind = NewObjectIdentifierVarbind(oid, value.(ObjectIdentifier))
	case snmpBlockType_IP_ADDRESS:
		varbind = NewIPv4AddressVarbind(oid, value.(net.IP))
	// case snmpBlockType_COUNTER_32:
	// 	varbind = NewCounter32Varbind(oid)
	// case snmpBlockType_GAUGE_32:
	// 	varbind = NewGauge32Varbind(oid)
	// case TIME_TICKS:
	// 	varbind = NewTimeTicksVarbind(oid)
	// case OPAQUE:
	// 	varbind = NewOpaqueVarbind(oid)
	// case COUNTER_64:
	// 	varbind = NewCounter64Varbind(oid)
	// case UINT_32:
	// 	varbind = NewUint32Varbind(oid)
	case snmpBlockType_NO_SUCH_OBJECT:
		varbind = NewNoSuchObjectVarbind(oid)
	case snmpBlockType_NO_SUCH_INSTANCE:
		varbind = NewNoSuchInstanceVarbindVarbind(oid)
	case snmpBlockType_END_OF_MIB_VIEW:
		varbind = NewEndOfMibViewVarbind(oid)
	default:
		return nil, fmt.Errorf("Unknown value type 0x%x", valueType)
	}
	if startDecoderLen-decoder.Len() != varbindLength {
		return nil, fmt.Errorf("Decoding varbind consumed too many bytes. Expected: %d, actual: %d", varbindLength, startDecoderLen-decoder.Len())
	}
	return
}
