package snmp_go

import (
	"errors"
	"fmt"
	. "github.com/idawes/ber_go"
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
	encode(encoderFactory *BerEncoderFactory) []byte
	getAddress() *net.UDPAddr
	setAddress(*net.UDPAddr)
	GetLoggingId() string
}

type SnmpRequest interface {
	SnmpMessage
	AddOid(oid []uint32)
	AddOids(oids [][]uint32)
	GetFlightTime() time.Duration
	getResponseHandler() chan<- SnmpRequest
	setRequestId(requestId uint32)
	getRequestId() uint32
	resetRetryCount()
	isRetryRequired() bool
	startTimer(expiredMsgChannel chan<- SnmpRequest)
	stopTimer()
	setResponse(resp SnmpResponse)
	setError(err error)
}

type SnmpResponse interface {
	getRequestId() uint32
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

func decodeCommunityMessage(decoder *BerDecoder, version SnmpVersion) (msg SnmpMessage, err error) {
	communityBytes, err := decoder.DecodeOctetString()
	if err != nil {
		return nil, err
	}
	community := string(communityBytes)
	rawPduType, pduLength, err := decoder.DecodeHeader()
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Unabled to decode pdu header - err: %s", err))
	}
	if pduLength != decoder.Len() {
		return nil, errors.New(fmt.Sprintf("Encoded pdu length %d doesn't match remaining msg length %d", pduLength, decoder.Len()))
	}
	pduType := PDUType(rawPduType)
	switch pduType {
	case GET_REQUEST, GET_RESPONSE, GET_NEXT_REQUEST, SET_REQUEST:
		_msg := new(communityRequestResponse)
		_msg.version = version
		_msg.community = community
		_msg.pduType = pduType
		_msg.decode(decoder)
		msg = _msg
	case GET_BULK_REQUEST, INFORM_REQUEST, V2_TRAP, REPORT:
		if version == Version1 {
			return nil, errors.New(fmt.Sprintf("Invalid PDU type for SNMP version 1 message: %s", pduType))
		}
		_msg := new(communityRequestResponse)
		_msg.version = version
		_msg.community = community
		_msg.pduType = pduType
		_msg.decode(decoder)
		msg = _msg
	case V1_TRAP:
		if version != Version1 {
			return nil, errors.New(fmt.Sprintf("Invalid version for V1 Trap message: %s", pduType, version))
		}
		_msg := new(V1Trap)
		_msg.version = version
		_msg.community = community
		_msg.pduType = V1_TRAP
		_msg.decode(decoder)
		msg = _msg
	default:
		return nil, errors.New(fmt.Sprintf("Unsupported PDU type: 0x%x", rawPduType))
	}
	return
}

// base type for all v1/v2c messages other than v1Trap
type communityRequestResponse struct {
	communityMessage
	requestId uint32
	errorVal  int32
	errorIdx  int32
}

func (msg *communityRequestResponse) GetLoggingId() string {
	return fmt.Sprintf("%s:%d", msg.pduType.String(), msg.requestId)
}

func (msg *communityRequestResponse) setRequestId(requestId uint32) {
	msg.requestId = requestId
}

func (msg *communityRequestResponse) getRequestId() uint32 {
	return msg.requestId
}

func (msg *communityRequestResponse) encode(encoderFactory *BerEncoderFactory) []byte {
	encoder := encoderFactory.NewBerEncoder()
	defer encoder.Destroy()
	msgHeader := encoder.NewHeader(SEQUENCE)
	headerFieldsLen := encoder.EncodeInteger(int64(msg.version))
	headerFieldsLen += encoder.EncodeOctetString([]byte(msg.community))
	headerFieldsLen += encoder.EncodeInteger(int64(msg.requestId))
	headerFieldsLen += encoder.EncodeInteger(int64(msg.errorVal))
	headerFieldsLen += encoder.EncodeInteger(int64(msg.errorIdx))
	pduHeader := encoder.NewHeader(byte(msg.pduType))
	varbindsListHeader := encoder.NewHeader(SEQUENCE)
	varbindsLen := 0
	for _, varbind := range msg.varbinds {
		varbindsLen += varbind.encode(encoder)
	}
	_, pduLen := varbindsListHeader.SetContentLength(varbindsLen)
	_, msgLen := pduHeader.SetContentLength(pduLen)
	msgLen += headerFieldsLen
	msgHeader.SetContentLength(msgLen)
	return encoder.Serialize()
}

func (msg *communityRequestResponse) decode(decoder *BerDecoder) (err error) {
	return
}

type CommunityRequest struct {
	communityRequestResponse
	response        SnmpResponse
	timeoutSeconds  int
	retries         int
	retryCount      int
	timer           *time.Timer
	timeoutChan     chan<- SnmpRequest
	responseHandler chan<- SnmpRequest
	flightStartTime time.Time
	flightTime      time.Duration
	err             error
}

func (req *CommunityRequest) GetFlightTime() time.Duration {
	return req.flightTime
}

func (req *CommunityRequest) startTimer(timeoutChan chan<- SnmpRequest) {
	req.timeoutChan = timeoutChan
	req.flightStartTime = time.Now()
	req.timer = time.AfterFunc(time.Duration(req.timeoutSeconds)*time.Second, req.handleTimeout)
}

func (req *CommunityRequest) stopTimer() {
	req.timer.Stop()
	req.flightTime = time.Since(req.flightStartTime)
}

func (req *CommunityRequest) handleTimeout() {
	req.flightTime = time.Since(req.flightStartTime)
	req.timeoutChan <- req
}

func (req *CommunityRequest) resetRetryCount() {
	req.retryCount = 0
}

func (req *CommunityRequest) isRetryRequired() bool {
	req.retryCount += 1
	return req.retryCount <= req.retries
}

func (req *CommunityRequest) AddOid(oid []uint32) {
	req.varbinds = append(req.varbinds, NewNullVarbind(oid))
}

func (req *CommunityRequest) AddOids(oids [][]uint32) {
	temp := make([]Varbind, len(oids))
	for i, oid := range oids {
		temp[i] = NewNullVarbind(oid)
	}
	req.varbinds = append(req.varbinds, temp...)
}

func (req *CommunityRequest) getResponseHandler() chan<- SnmpRequest {
	return req.responseHandler
}

func (req *CommunityRequest) setResponse(resp SnmpResponse) {
	req.response = resp
}

func (req *CommunityRequest) setError(err error) {
	req.err = err
}

func (req *CommunityRequest) GetResponse() (resp SnmpResponse) {
	return req.response
}

func (req *CommunityRequest) GetError() (err error) {
	return req.err
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

func (msg *V1Trap) GetLoggingId() string {
	return fmt.Sprintf("%s:%d", msg.pduType, msg.timeStamp)
}

func (msg *V1Trap) encode(encoderFactory *BerEncoderFactory) []byte {
	encoder := encoderFactory.NewBerEncoder()
	defer encoder.Destroy()
	msgHeader := encoder.NewHeader(SEQUENCE)
	headerFieldsLen := encoder.EncodeInteger(int64(msg.version))
	headerFieldsLen += encoder.EncodeOctetString([]byte(msg.community))
	pduHeader := encoder.NewHeader(byte(msg.pduType))
	varbindsListHeader := encoder.NewHeader(SEQUENCE)
	varbindsLen := 0
	for _, varbind := range msg.varbinds {
		varbindsLen += varbind.encode(encoder)
	}
	_, pduLen := varbindsListHeader.SetContentLength(varbindsLen)
	_, msgLen := pduHeader.SetContentLength(pduLen)
	msgLen += headerFieldsLen
	msgHeader.SetContentLength(msgLen)
	return encoder.Serialize()
}

func (msg *V1Trap) decode(decoder *BerDecoder) (err error) {
	return
}

func decodeMsg(rawMsg []byte) (decodedMsg SnmpMessage, err error) {
	decoder := NewBerDecoder(rawMsg)
	msgType, length, err := decoder.DecodeHeader()
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Unable to decode message header - err: %s", err))
	}
	if msgType != SEQUENCE {
		return nil, errors.New(fmt.Sprintf("Invalid message header type 0x%x - not 0x%x", msgType, SEQUENCE))
	}
	if length != decoder.Len() {
		return nil, errors.New(fmt.Sprintf("Invalid message length - expected %d, got %d", length, decoder.Len()))
	}
	rawVersion, err := decoder.DecodeInteger()
	if err != nil {
		return nil, err
	}
	version := SnmpVersion(rawVersion)
	switch version {
	case Version1, Version2c:
		return decodeCommunityMessage(decoder, version)
	default:
		return nil, errors.New(fmt.Sprintf("Unsupported snmp version code 0x%x", version))
	}
}
