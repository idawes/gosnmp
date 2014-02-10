package agent_support

import (
	"fmt"
	. "github.com/idawes/gosnmp"
)

type objectNotWriteableError struct {
	oid ObjectIdentifier
}

func (e objectNotWriteableError) Error() string {
	return fmt.Sprintf("Object Not Writeable: %v", e.oid)
}

type incorrectVarbindTypeError struct {
	vb       Varbind
	expected Varbind
}

func (e incorrectVarbindTypeError) Error() string {
	return fmt.Sprintf("Incorrect varbind type for: %v, got: %T, expecting: %d", e.vb.GetOid(), e.vb, e.expected)
}

type basicOidHandler struct {
	writable bool
}

func (handler *basicOidHandler) SetWritable(writable bool) {
	handler.writable = writable
}

func (handler *basicOidHandler) Writable() bool {
	return handler.writable
}

// IntOidHandler implements a very simple handler serving up a single int32 variable, and allowing non-transaction based
// updates of that value
type IntOidHandler struct {
	basicOidHandler
	val int32
}

func NewIntOidHandler(val int32, writable bool) *IntOidHandler {
	handler := new(IntOidHandler)
	handler.val = val
	handler.writable = writable
	return handler
}

func (handler *IntOidHandler) Get(oid ObjectIdentifier) (Varbind, error) {
	return NewIntegerVarbind(oid, handler.val), nil
}

func (handler *IntOidHandler) Set(vb_base Varbind) error {
	if !handler.writable {
		return objectNotWriteableError{}
	}
	vb, ok := vb_base.(*IntegerVarbind)
	if !ok {
		return incorrectVarbindTypeError{vb_base, new(IntegerVarbind)}
	}
	handler.val = vb.Value
	return nil
}

// OctetStringOidHandler implements a very simple handler serving up a single []byte variable, and allowing non-
// transaction based updates of that value. This is also the correct simple handler for a string value
type OctetStringOidHandler struct {
	basicOidHandler
	val []byte
}

func NewStringOidHandler(val string, writable bool) *OctetStringOidHandler {
	return NewOctetStringOidHandler([]byte(val), writable)
}

func NewOctetStringOidHandler(val []byte, writable bool) *OctetStringOidHandler {
	if val == nil {
		panic("value must be specified")
	}
	handler := new(OctetStringOidHandler)
	handler.val = val
	handler.writable = writable
	return handler
}

func (handler *OctetStringOidHandler) Get(oid ObjectIdentifier, txn interface{}) (Varbind, error) {
	return NewOctetStringVarbind(oid, handler.val), nil
}

func (handler *OctetStringOidHandler) Set(vb_base Varbind, txn interface{}) (Varbind, error) {
	if !handler.writable {
		return nil, objectNotWriteableError{}
	}
	vb, ok := vb_base.(*OctetStringVarbind)
	if !ok {
		return nil, incorrectVarbindTypeError{vb_base, new(OctetStringVarbind)}
	}
	if vb.Value == nil {
		panic(fmt.Sprintf("value must be specified: GetOid(): %v", vb.GetOid()))
	}
	handler.val = vb.Value
	return vb, nil
}

// ObjectIdentifierOidHandler implements a very simple handler serving up a single ObjectIdentifer variable, and allowing non-
// transaction based updates of that value.
type ObjectIdentifierOidHandler struct {
	basicOidHandler
	val ObjectIdentifier
}

func NewObjectIdentifierOidHandler(val ObjectIdentifier, writable bool) *ObjectIdentifierOidHandler {
	if val == nil {
		panic("value must be specified")
	}
	handler := new(ObjectIdentifierOidHandler)
	handler.val = val
	handler.writable = writable
	return handler
}

func (handler *ObjectIdentifierOidHandler) Get(oid ObjectIdentifier, txn interface{}) (Varbind, error) {
	return NewObjectIdentifierVarbind(oid, handler.val), nil
}

func (handler *ObjectIdentifierOidHandler) Set(vb_base Varbind, txn interface{}) (Varbind, error) {
	if !handler.writable {
		return nil, objectNotWriteableError{}
	}
	vb, ok := vb_base.(*ObjectIdentifierVarbind)
	if !ok {
		return nil, incorrectVarbindTypeError{vb_base, new(ObjectIdentifierVarbind)}
	}
	if vb.Value == nil {
		panic(fmt.Sprintf("value must be specified: GetOid(): %v", vb.GetOid()))
	}
	handler.val = vb.Value
	return vb, nil
}
