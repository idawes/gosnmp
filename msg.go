package snmp_go

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"time"
)

type PDUType byte

const (
	GET_REQUEST      PDUType = 0xa0
	GET_NEXT_REQUEST         = 0xa1
	GET_RESPONSE             = 0xa2
	SET_REQUEST              = 0xa3
	V1_TRAP                  = 0xa4
	GET_BULK_REQUEST         = 0xa5
	INFORM_REQUEST           = 0xa6
	V2_TRAP                  = 0xa7
	REPORT                   = 0xa8
)

func (pduType *PDUType) String() string {
	switch *pduType {
	case GET_REQUEST:
		return "GET REQUEST"
	case GET_NEXT_REQUEST:
		return "GET NEXT REQUEST"
	case GET_RESPONSE:
		return "GET RESPONSE"
	case SET_REQUEST:
		return "SET REQUEST"
	default:
		return "UNKNOWN PDU TYPE"
	}
}

type SnmpMessage interface {
	marshal(bufferPool *bufferPool) []byte
	getAddress() *net.UDPAddr
	setAddress(*net.UDPAddr)
}

type SnmpRequest interface {
	SnmpMessage
	AddOid(oid []int32)
	AddOids(oids [][]int32)
	ProcessResponse(SnmpResponse, error)
	setRequestId(requestId int32)
	getRequestId() int32
	resetRetryCount()
	isRetryRequired() bool
	startTimer(expiredMsgChannel chan<- SnmpRequest)
}

type SnmpResponse interface {
	getRequestId() int32
}

type V2cMessage interface {
}

// base type for all SNMP messages
type baseMsg struct {
	version  SnmpVersion
	pduType  PDUType
	varbinds []Varbind
	address  *net.UDPAddr
}

func (msg *baseMsg) getAddress() *net.UDPAddr {
	return msg.address
}

func (msg *baseMsg) setAddress(addr *net.UDPAddr) {
	msg.address = addr
}

// base type for all v1/v2c messages
type communityMessage struct {
	baseMsg
	community string
}

func unmarshalCommunityMsg(buf *bytes.Buffer, version SnmpVersion) (msg SnmpMessage, err error) {
	communityBytes, err := unmarshalOctetString(buf)
	if err != nil {
		return nil, err
	}
	community := string(communityBytes)
	rawPduType, pduLength, err := unmarshalTypeAndLength(buf)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Unabled to decode pdu header - err: %s", err))
	}
	if pduLength != buf.Len() {
		return nil, errors.New(fmt.Sprintf("Invalid pdu length - expected %d, got %d", pduLength, buf.Len()))
	}
	pduType := PDUType(rawPduType)
	switch pduType {
	case GET_REQUEST, GET_RESPONSE, GET_NEXT_REQUEST, SET_REQUEST:
		_msg := new(communityRequestResponse)
		_msg.version = version
		_msg.community = community
		_msg.pduType = pduType
		_msg.unmarshal(buf)
		msg = _msg
	case GET_BULK_REQUEST, INFORM_REQUEST, V2_TRAP, REPORT:
		if version == Version1 {
			return nil, errors.New(fmt.Sprintf("Invalid PDU type for SNMP version 1 message: %s", pduType))
		}
		_msg := new(communityRequestResponse)
		_msg.version = version
		_msg.community = community
		_msg.pduType = pduType
		_msg.unmarshal(buf)
		msg = _msg
	case V1_TRAP:
		if version != Version1 {
			return nil, errors.New(fmt.Sprintf("Invalid version for V1 Trap message: %s", pduType, version))
		}
		_msg := new(V1Trap)
		_msg.version = version
		_msg.community = community
		_msg.pduType = V1_TRAP
		msg = _msg
	default:
		return nil, errors.New(fmt.Sprintf("Unsupported PDU type: 0x%x", rawPduType))
	}
	return
}

// base type for all v1/v2c messages other than v1Trap
type communityRequestResponse struct {
	communityMessage
	requestId int32
	errorVal  int32
	errorIdx  int32
}

func (msg *communityRequestResponse) setRequestId(requestId int32) {
	msg.requestId = requestId
}

func (msg *communityRequestResponse) getRequestId() int32 {
	return msg.requestId
}

func (msg *communityRequestResponse) marshal(bufferPool *bufferPool) []byte {
	fmt.Printf("%v Marshalling msg %d\n", time.Now(), msg.requestId)
	chain := newBufferChain(bufferPool)
	defer chain.destroy()
	msgHeader := chain.addBufToTail()
	msgLen := marshalInteger(chain.addBufToTail(), chain.addBufToTail(), int64(msg.version))
	msgLen += marshalOctetString(chain.addBufToTail(), chain.addBufToTail(), []byte(msg.community))
	pduHeader := chain.addBufToTail()
	pduLen := marshalInteger(chain.addBufToTail(), chain.addBufToTail(), int64(msg.requestId))
	pduLen += marshalInteger(chain.addBufToTail(), chain.addBufToTail(), int64(msg.errorVal))
	pduLen += marshalInteger(chain.addBufToTail(), chain.addBufToTail(), int64(msg.errorIdx))
	varbindsListHeader := chain.addBufToTail()
	varbindsLen := 0
	for _, varbind := range msg.varbinds {
		varbindsLen += varbind.Marshal(chain)
	}
	pduLen += varbindsLen
	pduLen += marshalTypeAndLength(varbindsListHeader, SEQUENCE, varbindsLen)
	msgLen += pduLen
	msgLen += marshalTypeAndLength(pduHeader, byte(msg.pduType), pduLen)
	marshalTypeAndLength(msgHeader, SEQUENCE, msgLen)
	return chain.collapse()
}

func (msg *communityRequestResponse) unmarshal(buf *bytes.Buffer) (err error) {
	return
}

type CommunityRequest struct {
	communityRequestResponse
	response        *communityRequestResponse
	timeoutSeconds  int
	retries         int
	retryCount      int
	timer           *time.Timer
	timeoutChan     chan<- SnmpRequest
	responseHandler func(SnmpRequest, SnmpResponse, error)
	inFlight        bool
}

func (req *CommunityRequest) startTimer(timeoutChan chan<- SnmpRequest) {
	req.timeoutChan = timeoutChan
	req.timer = time.AfterFunc(time.Duration(req.timeoutSeconds)*time.Second, req.handleTimeout)
}

func (req *CommunityRequest) handleTimeout() {
	req.timeoutChan <- req
}

func (req *CommunityRequest) resetRetryCount() {
	req.retryCount = 0
}

func (req *CommunityRequest) isRetryRequired() bool {
	req.retryCount += 1
	return req.retryCount <= req.retries
}

func (req *CommunityRequest) AddOid(oid []int32) {
	req.varbinds = append(req.varbinds, NewNullVarbind(oid))
}

func (req *CommunityRequest) AddOids(oids [][]int32) {
	temp := make([]Varbind, len(oids))
	for i, oid := range oids {
		temp[i] = NewNullVarbind(oid)
	}
	req.varbinds = append(req.varbinds, temp...)
}

func (req *CommunityRequest) ProcessResponse(resp SnmpResponse, err error) {
	req.responseHandler(req, resp, err)
}

type CommunityResponse struct {
	communityRequestResponse
}

type V1Trap struct {
	communityMessage
	enterprise   []uint32
	agentAddr    *net.IPAddr
	genericTrap  uint32
	specificTrap uint32
	timeStamp    uint32
}

func (msg *V1Trap) marshal(bufferPool *bufferPool) []byte {
	chain := newBufferChain(bufferPool)
	defer chain.destroy()
	msgHeader := chain.addBufToTail()
	msgLen := marshalInteger(chain.addBufToTail(), chain.addBufToTail(), int64(msg.version))
	msgLen += marshalOctetString(chain.addBufToTail(), chain.addBufToTail(), []byte(msg.community))
	pduHeader := chain.addBufToTail()
	pduLen := 0
	// pduLen := marshalInteger(chain.addBufToTail(), chain.addBufToTail(), int64(msg.requestId))
	// pduLen += marshalInteger(chain.addBufToTail(), chain.addBufToTail(), int64(msg.errorVal))
	// pduLen += marshalInteger(chain.addBufToTail(), chain.addBufToTail(), int64(msg.errorIdx))
	varbindsListHeader := chain.addBufToTail()
	varbindsLen := 0
	for _, varbind := range msg.varbinds {
		varbindsLen += varbind.Marshal(chain)
	}
	pduLen += varbindsLen
	pduLen += marshalTypeAndLength(varbindsListHeader, SEQUENCE, varbindsLen)
	msgLen += pduLen
	msgLen += marshalTypeAndLength(pduHeader, byte(msg.pduType), pduLen)
	marshalTypeAndLength(msgHeader, SEQUENCE, msgLen)
	return chain.collapse()
}

func unmarshalMsg(buf *bytes.Buffer) (msg SnmpMessage, err error) {
	msgType, length, err := unmarshalTypeAndLength(buf)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Unable to decode message header - err: %s", err))
	}
	if msgType != SEQUENCE {
		return nil, errors.New(fmt.Sprintf("Invalid message header type - not 0x%x", SEQUENCE))
	}
	if length != buf.Len() {
		return nil, errors.New(fmt.Sprintf("Invalid message length - expected %d, got %d", length, buf.Len()))
	}
	rawVersion, err := unmarshalInteger(buf)
	if err != nil {
		return nil, err
	}
	version := SnmpVersion(rawVersion)
	switch version {
	case Version1, Version2c:
		return unmarshalCommunityMsg(buf, version)
	default:
		return nil, errors.New(fmt.Sprintf("Unsupported snmp version code 0x%x", version))
	}
}
