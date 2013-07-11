package snmp_go

import (
	"bytes"
	"errors"
	"fmt"
)

const (
	INTEGER           byte = 0x02
	BIT_STRING             = 0x03
	OCTET_STRING           = 0x04
	NULL                   = 0x05
	OBJECT_IDENTIFIER      = 0x06
	SEQUENCE               = 0x30
)

func marshalTypeAndLength(buf *bytes.Buffer, t byte, l int) int {
	buf.WriteByte(t)
	return 1 + marshalLength(buf, l)
}

func unmarshalTypeAndLength(buf *bytes.Buffer) (t byte, l int, err error) {
	t, err = buf.ReadByte()
	if err != nil {
		return 0, 0, err
	}
	l, err = unmarshalLength(buf)
	if err != nil {
		return 0, 0, err
	}
	return
}

func marshalLength(buf *bytes.Buffer, l int) int {
	if l < 127 {
		buf.WriteByte(byte(l))
		return 1
	} else {
		n := calculateLengthLen(l)
		buf.WriteByte(0x80 | n)
		for ; n > 0; n-- {
			buf.WriteByte(byte(l >> uint((n-1)*8)))
		}
		return int(n + 1)
	}
}

func calculateLengthLen(l int) byte {
	numBytes := 1
	for ; l > 255; l >>= 8 {
		numBytes++
	}
	return byte(numBytes)
}

func unmarshalLength(buf *bytes.Buffer) (l int, err error) {
	l = 0
	firstByte, err := buf.ReadByte()
	if err != nil {
		return 0, err
	}
	if firstByte < 127 {
		l = int(firstByte)
		return
	}
	numBytes := firstByte
	for ; numBytes > 0; numBytes-- {
		temp, err := buf.ReadByte()
		if err != nil {
			return 0, err
		}
		l <<= 8
		l += int(temp)
	}
	return
}

func marshalBase128Int(buf *bytes.Buffer, val int64) int {
	if val == 0 {
		buf.WriteByte(0)
		return 1
	}
	numBytesToWrite := calculateBase128IntLen(val)
	for i := numBytesToWrite - 1; i >= 0; i-- {
		byteToWrite := byte(val>>uint(i*7)) & 0x7f
		if i != 0 {
			byteToWrite |= 0x80
		}
		buf.WriteByte(byteToWrite)
	}
	return int(numBytesToWrite)
}

func calculateBase128IntLen(val int64) int {
	numBytes := 0
	for i := val; i > 0; i >>= 7 {
		numBytes++
	}
	return numBytes
}

func marshal2sComplementInt(buf *bytes.Buffer, val int64) int {
	numBytesToWrite := calculate2sComplementIntLen(val)
	for i := numBytesToWrite; i > 0; i-- {
		buf.WriteByte(byte(val >> uint((i-1)*8)))
	}
	return numBytesToWrite
}

func calculate2sComplementIntLen(val int64) int {
	numBytes := 1
	for ; val > 127; val >>= 8 {
		numBytes++
	}
	for ; val < -128; val >>= 8 {
		numBytes++
	}
	return numBytes
}

func unmarshal2sComplementInt(buf *bytes.Buffer, numBytes int) (val int64, err error) {
	for i := 0; i < numBytes; i++ {
		temp, err := buf.ReadByte()
		if err != nil {
			return 0, err
		}
		val <<= 8
		val |= int64(temp)
	}

	// Shift up and down in order to sign extend the result.
	val <<= 64 - uint8(numBytes)*8
	val >>= 64 - uint8(numBytes)*8
	return
}

func marshalInteger(headerBuf, valBuf *bytes.Buffer, val int64) int {
	numWritten := marshal2sComplementInt(valBuf, val)
	return numWritten + marshalTypeAndLength(headerBuf, INTEGER, numWritten)
}

func unmarshalInteger(buf *bytes.Buffer) (val int64, err error) {
	t, l, err := unmarshalTypeAndLength(buf)
	if err != nil {
		return 0, err
	}
	if t != INTEGER {
		return 0, errors.New(fmt.Sprintf("Expecting type %x, found %x", INTEGER, t))
	}
	val, err = unmarshal2sComplementInt(buf, l)
	return
}

func marshalOctetString(headerBuf, valBuf *bytes.Buffer, val []byte) int {
	numWritten, _ := valBuf.Write(val)
	return numWritten + marshalTypeAndLength(headerBuf, OCTET_STRING, numWritten)
}

func unmarshalOctetString(buf *bytes.Buffer) (val []byte, err error) {
	t, l, err := unmarshalTypeAndLength(buf)
	if err != nil {
		return nil, err
	}
	if t != OCTET_STRING {
		return nil, errors.New(fmt.Sprintf("Expecting type %x, found %x", OCTET_STRING, t))
	}
	if l > buf.Len() {
		return nil, errors.New(fmt.Sprintf("Invalid length %d for octet string"))
	}
	val = make([]byte, l)
	if numRead, err := buf.Read(val); err != nil || numRead != l {
		return nil, errors.New(fmt.Sprintf("Couldn't decode octet string of length %d, numRead: %d, err: %s", l, numRead, err))
	}
	return
}

func marshalObjectIdentifier(headerBuf, valBuf *bytes.Buffer, oid []int32) int {
	if len(oid) < 2 || oid[0] > 6 || oid[1] >= 40 {
		panic(fmt.Sprintf("Invalid oid: %v", oid))
	}
	numWritten := 1
	valBuf.WriteByte(byte(oid[0]*40 + oid[1])) // first byte holds the first two identifiers in the oid
	for i := 2; i < len(oid); i++ {            // remaining oid identifiers are marshalled as base 128 integers
		numWritten += marshalBase128Int(valBuf, int64(oid[i]))
	}
	return numWritten + marshalTypeAndLength(headerBuf, OBJECT_IDENTIFIER, numWritten)
}
