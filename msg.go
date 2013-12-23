package gosnmp

import (
	"fmt"
	"net"
	"time"
)

type pduType snmpBlockType

const (
	pduType_GET_REQUEST      pduType = 0xa0
	pduType_GET_NEXT_REQUEST         = 0xa1
	pduType_GET_RESPONSE             = 0xa2
	pduType_SET_REQUEST              = 0xa3
	pduType_V1_TRAP                  = 0xa4
	pduType_GET_BULK_REQUEST         = 0xa5
	pduType_INFORM_REQUEST           = 0xa6
	pduType_V2_TRAP                  = 0xa7
	pduType_REPORT                   = 0xa8
)

func (pduType *pduType) String() string {
	switch *pduType {
	case pduType_GET_REQUEST:
		return "GET REQUEST"
	case pduType_GET_NEXT_REQUEST:
		return "GET NEXT REQUEST"
	case pduType_GET_RESPONSE:
		return "GET RESPONSE"
	case pduType_SET_REQUEST:
		return "SET REQUEST"
	default:
		return "UNKNOWN PDU TYPE"
	}
}

type SnmpMessage interface {
	encode(encoderFactory *berEncoderFactory) ([]byte, error)
	decode(decoder *berDecoder) error
	getAddress() *net.UDPAddr
	setAddress(*net.UDPAddr)
	getVersion() SnmpVersion
	setVersion(version SnmpVersion)
	getpduType() pduType
	setpduType(pduType pduType)
	GetLoggingId() string
}

type SnmpRequest interface {
	SnmpMessage
	AddOid(oid ObjectIdentifier)
	AddOids(oids []ObjectIdentifier)
	GetFlightTime() time.Duration
	wait()
	notify()
	setRequestId(requestId uint32)
	getRequestId() uint32
	isRetryRequired() bool
	startTimer(func(SnmpRequest))
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
	pduType  pduType
	varbinds []Varbind
	address  *net.UDPAddr
}

func (msg *baseMsg) getVersion() SnmpVersion {
	return msg.version
}

func (msg *baseMsg) setVersion(version SnmpVersion) {
	msg.version = version
}

func (msg *baseMsg) getpduType() pduType {
	return msg.pduType
}

func (msg *baseMsg) setpduType(pduType pduType) {
	msg.pduType = pduType
}

func (msg *baseMsg) getAddress() *net.UDPAddr {
	return msg.address
}

func (msg *baseMsg) setAddress(addr *net.UDPAddr) {
	msg.address = addr
}

func (msg *baseMsg) decodeVarbinds(decoder *berDecoder) (err error) {
	varbindsListType, varbindsListLength, err := decoder.decodeHeader()
	if err != nil {
		return fmt.Errorf("Unable to decode varbinds list header - err: %s", err)
	}
	if varbindsListType != snmpBlockType_SEQUENCE {
		return fmt.Errorf("Invalid message header type 0x%x - not 0x%x", varbindsListType, snmpBlockType_SEQUENCE)
	}
	if varbindsListLength != decoder.Len() {
		return fmt.Errorf("Encoded varbinds list length %d doesn't match remaining msg length %d", varbindsListLength, decoder.Len())
	}
	varbindCount := 1
	for ; ; varbindCount++ {
		varbind, err := decodeVarbind(decoder)
		if err != nil {
			return fmt.Errorf("Decoding of varbind %d failed - err: %s", varbindCount, err)
		}
		msg.varbinds = append(msg.varbinds, varbind)
	}
}

// base type for all v1/v2c messages
type communityMessage struct {
	baseMsg
	community string
}

type snmpCommunityMessage interface {
	SnmpMessage
	getCommunity() string
	setCommunity(community string)
}

func (msg *communityMessage) getCommunity() string {
	return msg.community
}

func (msg *communityMessage) setCommunity(community string) {
	msg.community = community
}

func decodeCommunityMessage(decoder *berDecoder, version SnmpVersion) (snmpCommunityMessage, error) {
	communityBytes, err := decoder.decodeOctetStringWithHeader()
	if err != nil {
		return nil, err
	}
	community := string(communityBytes)
	rawpduType, pduLength, err := decoder.decodeHeader()
	if err != nil {
		return nil, fmt.Errorf("Unabled to decode pdu header - err: %s", err)
	}
	if pduLength != decoder.Len() {
		return nil, fmt.Errorf("Encoded pdu length %d doesn't match remaining msg length %d", pduLength, decoder.Len())
	}
	pduType := pduType(rawpduType)
	var msg snmpCommunityMessage
	switch pduType {
	case pduType_GET_REQUEST, pduType_GET_NEXT_REQUEST, pduType_SET_REQUEST:
		msg = new(CommunityRequest)
	case pduType_GET_RESPONSE:
		msg = new(CommunityResponse)
	case pduType_GET_BULK_REQUEST, pduType_INFORM_REQUEST, pduType_V2_TRAP, pduType_REPORT:
		if version == Version1 {
			return nil, fmt.Errorf("Invalid PDU type for SNMP version 1 message: %s", pduType)
		}
		switch pduType {
		case pduType_GET_BULK_REQUEST:
			msg = new(CommunityRequest)
		case pduType_INFORM_REQUEST, pduType_V2_TRAP, pduType_REPORT:
			return nil, fmt.Errorf("PDU type %d not supported yet", pduType)
		}
	case pduType_V1_TRAP:
		if version != Version1 {
			return nil, fmt.Errorf("Invalid version for V1 Trap message: %s", version)
		}
		msg = new(V1Trap)
	default:
		return nil, fmt.Errorf("Unsupported PDU type: 0x%x", rawpduType)
	}
	msg.setVersion(version)
	msg.setCommunity(community)
	msg.setpduType(pduType)
	msg.decode(decoder)
	return msg, nil
}

// base type for all v1/v2c request/response messages
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

func (msg *communityRequestResponse) encode(encoderFactory *berEncoderFactory) ([]byte, error) {
	encoder := encoderFactory.newberEncoder()
	defer encoder.destroy()
	msgHeader := encoder.newHeader(snmpBlockType_SEQUENCE)
	headerFieldsLen := encoder.encodeInteger(int64(msg.version))
	headerFieldsLen += encoder.encodeOctetString([]byte(msg.community))
	pduHeader := encoder.newHeader(snmpBlockType(msg.pduType))
	pduControlFieldsLen := encoder.encodeInteger(int64(msg.requestId))
	pduControlFieldsLen += encoder.encodeInteger(int64(msg.errorVal))
	pduControlFieldsLen += encoder.encodeInteger(int64(msg.errorIdx))
	varbindsListHeader := encoder.newHeader(snmpBlockType_SEQUENCE)
	varbindsLen := 0
	for _, varbind := range msg.varbinds {
		encodedLen, err := encoder.encodeVarbind(varbind)
		if err != nil {
			return nil, err
		}
		varbindsLen += encodedLen
	}
	_, varbindsListLen := varbindsListHeader.setContentLength(varbindsLen)
	_, pduLen := pduHeader.setContentLength(pduControlFieldsLen + varbindsListLen)
	msgHeader.setContentLength(headerFieldsLen + pduLen)
	return encoder.serialize(), nil
}

func (msg *communityRequestResponse) decode(decoder *berDecoder) error {
	var err error
	if msg.requestId, err = decoder.decodeUint32WithHeader(); err != nil {
		return err
	}
	if msg.errorVal, err = decoder.decodeInt32WithHeader(); err != nil {
		return err
	}
	if msg.errorIdx, err = decoder.decodeInt32WithHeader(); err != nil {
		return err
	}
	return msg.decodeVarbinds(decoder)
}

type CommunityRequest struct {
	communityRequestResponse
	response         SnmpResponse
	timeoutSeconds   int
	retriesRemaining int
	timer            *time.Timer
	timeoutFunc      func(SnmpRequest)
	requestDoneChan  chan bool
	flightStartTime  time.Time
	flightTime       time.Duration
	err              error
}

func newCommunityRequest() *CommunityRequest {
	req := new(CommunityRequest)
	req.requestDoneChan = make(chan bool)
	return req
}

func (req *CommunityRequest) GetFlightTime() time.Duration {
	return req.flightTime
}

func (req *CommunityRequest) startTimer(timeoutFunc func(SnmpRequest)) {
	req.timeoutFunc = timeoutFunc
	req.flightStartTime = time.Now()
	req.timer = time.AfterFunc(time.Duration(req.timeoutSeconds)*time.Second, req.handleTimeout)
}

func (req *CommunityRequest) stopTimer() {
	req.timer.Stop()
	req.flightTime = time.Since(req.flightStartTime)
}

func (req *CommunityRequest) handleTimeout() {
	req.flightTime = time.Since(req.flightStartTime)
	req.timeoutFunc(req)
}

func (req *CommunityRequest) isRetryRequired() bool {
	if req.retriesRemaining > 0 {
		req.retriesRemaining--
		return true
	}
	return false
}

func (req *CommunityRequest) AddOid(oid ObjectIdentifier) {
	req.varbinds = append(req.varbinds, NewNullVarbind(oid))
}

func (req *CommunityRequest) AddOids(oids []ObjectIdentifier) {
	temp := make([]Varbind, len(oids))
	for i, oid := range oids {
		temp[i] = NewNullVarbind(oid)
	}
	req.varbinds = append(req.varbinds, temp...)
}

func (req *CommunityRequest) wait() {
	<-req.requestDoneChan
}

func (req *CommunityRequest) notify() {
	req.requestDoneChan <- true
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

func (req *CommunityRequest) GetRequestType() (requestType pduType) {
	return req.pduType
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

func (msg *V1Trap) encode(encoderFactory *berEncoderFactory) ([]byte, error) {
	encoder := encoderFactory.newberEncoder()
	defer encoder.destroy()
	msgHeader := encoder.newHeader(snmpBlockType_SEQUENCE)
	headerFieldsLen := encoder.encodeInteger(int64(msg.version))
	headerFieldsLen += encoder.encodeOctetString([]byte(msg.community))
	pduHeader := encoder.newHeader(snmpBlockType(msg.pduType))
	varbindsListHeader := encoder.newHeader(snmpBlockType_SEQUENCE)
	varbindsLen := 0
	for _, varbind := range msg.varbinds {
		encodedLen, err := encoder.encodeVarbind(varbind)
		if err != nil {
			return nil, err
		}
		varbindsLen += encodedLen
	}
	_, pduLen := varbindsListHeader.setContentLength(varbindsLen)
	_, msgLen := pduHeader.setContentLength(pduLen)
	msgLen += headerFieldsLen
	msgHeader.setContentLength(msgLen)
	return encoder.serialize(), nil
}

func (msg *V1Trap) decode(decoder *berDecoder) (err error) {
	return
}

func decodeMsg(rawMsg []byte) (decodedMsg SnmpMessage, err error) {
	decoder := newberDecoder(rawMsg)
	msgType, length, err := decoder.decodeHeader()
	if err != nil {
		return nil, fmt.Errorf("Unable to decode message header - err: %s", err)
	}
	if msgType != snmpBlockType_SEQUENCE {
		return nil, fmt.Errorf("Invalid message header type 0x%x - not 0x%x", msgType, snmpBlockType_SEQUENCE)
	}
	if length != decoder.Len() {
		return nil, fmt.Errorf("Invalid message length - expected %d, got %d", length, decoder.Len())
	}
	rawVersion, err := decoder.decodeIntegerWithHeader()
	if err != nil {
		return nil, err
	}
	version := SnmpVersion(rawVersion)
	switch version {
	case Version1, Version2c:
		return decodeCommunityMessage(decoder, version)
	default:
		return nil, fmt.Errorf("Unsupported snmp version code 0x%x", version)
	}
}
