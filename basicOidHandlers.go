package gosnmp

import (
	"fmt"
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
	return fmt.Sprintf("Incorrect varbind type for: %v, got: %T, expecting: %d", e.vb.getOid(), e.vb, e.expected)
}

type IntOidHandler struct {
	val      int32
	writable bool
}

func NewIntOidHandler(val int32, writable bool) *IntOidHandler {
	return &IntOidHandler{val, writable}
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
		return incorrectVarbindTypeError{vb_base, &IntegerVarbind{}}
	}
	handler.val = vb.val
	return nil
}

type OctetStringOidHandler struct {
	val      []byte
	writable bool
}

func NewStringOidHandler(val string, writable bool) *OctetStringOidHandler {
	return &OctetStringOidHandler{[]byte(val), writable}
}

func NewOctetStringOidHandler(val []byte, writable bool) *OctetStringOidHandler {
	return &OctetStringOidHandler{val, writable}
}

func (handler *OctetStringOidHandler) Get(oid ObjectIdentifier) (Varbind, error) {
	return NewOctetStringVarbind(oid, handler.val), nil
}

func (handler *OctetStringOidHandler) Set(vb_base Varbind) error {
	if !handler.writable {
		return objectNotWriteableError{}
	}
	vb, ok := vb_base.(*OctetStringVarbind)
	if !ok {
		return incorrectVarbindTypeError{vb_base, &OctetStringVarbind{}}
	}
	handler.val = vb.val
	return nil
}

type ObjectIdentifierOidHandler struct {
	val      ObjectIdentifier
	writable bool
}

func NewObjectIdentifierOidHandler(val ObjectIdentifier, writable bool) *ObjectIdentifierOidHandler {
	return &ObjectIdentifierOidHandler{val, writable}
}

func (handler *ObjectIdentifierOidHandler) Get(oid ObjectIdentifier) (Varbind, error) {
	return NewObjectIdentifierVarbind(oid, handler.val), nil
}

func (handler *ObjectIdentifierOidHandler) Set(vb_base Varbind) error {
	if !handler.writable {
		return objectNotWriteableError{}
	}
	vb, ok := vb_base.(*ObjectIdentifierVarbind)
	if !ok {
		return incorrectVarbindTypeError{vb_base, &ObjectIdentifierVarbind{}}
	}
	handler.val = vb.val
	return nil
}
