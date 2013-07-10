package snmp_go

import (
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
)

type SnmpMessage interface {
	marshal(bufferPool *bufferPool) []byte
	setRequestId(requestId int32)
	getRequestId() int32
	getTargetAddress() *net.UDPAddr
	startTimer(expiredMsgChannel chan<- SnmpRequest)
}

type SnmpRequest interface {
	SnmpMessage
	AddOid(oid []int32)
	AddOids(oids [][]int32)
	ProcessResponse(SnmpResponse, error)
	resetRetryCount()
	isRetryRequired() bool
}

type SnmpResponse interface {
}

type V2cMessage interface {
}

type baseMsg struct {
	Version       SnmpVersion
	Type          PDUType
	Varbinds      []Varbind
	requestId     int32
	Error         int32
	ErrorIdx      int32
	targetAddress *net.UDPAddr
}

func (msg *baseMsg) setRequestId(requestId int32) {
	msg.requestId = requestId
}

func (msg *baseMsg) getRequestId() int32 {
	return msg.requestId
}

func (msg *baseMsg) getTargetAddress() *net.UDPAddr {
	return msg.targetAddress
}

type v2cMessage struct {
	baseMsg
	community string
}

func (msg *v2cMessage) marshal(bufferPool *bufferPool) []byte {
	fmt.Printf("%v Marshalling msg %d\n", time.Now(), msg.requestId)
	chain := newBufferChain(bufferPool)
	defer chain.destroy()
	msgHeader := chain.addBufToTail()
	msgLen := marshalInteger(chain.addBufToTail(), chain.addBufToTail(), int64(msg.Version))
	msgLen += marshalOctetString(chain.addBufToTail(), chain.addBufToTail(), []byte(msg.community))
	pduHeader := chain.addBufToTail()
	pduLen := marshalInteger(chain.addBufToTail(), chain.addBufToTail(), int64(msg.requestId))
	pduLen += marshalInteger(chain.addBufToTail(), chain.addBufToTail(), int64(msg.Error))
	pduLen += marshalInteger(chain.addBufToTail(), chain.addBufToTail(), int64(msg.ErrorIdx))
	varbindsListHeader := chain.addBufToTail()
	varbindsLen := 0
	for _, varbind := range msg.Varbinds {
		varbindsLen += varbind.Marshal(chain)
	}
	pduLen += varbindsLen
	pduLen += marshalTypeAndLength(varbindsListHeader, SEQUENCE, varbindsLen)
	msgLen += pduLen
	msgLen += marshalTypeAndLength(pduHeader, byte(msg.Type), pduLen)
	marshalTypeAndLength(msgHeader, SEQUENCE, msgLen)
	return chain.collapse()
}

type V2cRequest struct {
	v2cMessage
	response        *v2cMessage
	TimeoutSeconds  int
	Retries         int
	retryCount      int
	timer           *time.Timer
	timeoutChan     chan<- SnmpRequest
	responseHandler func(SnmpRequest, SnmpResponse, error)
	inFlight        bool
}

func NewV2cGetRequest() *V2cRequest {
	req := new(V2cRequest)
	req.Version = Version2c
	req.Type = GET_REQUEST
	return req
}

func NewV2cGetNextRequest() *V2cRequest {
	req := new(V2cRequest)
	req.Version = Version2c
	req.Type = GET_NEXT_REQUEST
	return req
}

func (req *V2cRequest) startTimer(timeoutChan chan<- SnmpRequest) {
	req.timeoutChan = timeoutChan
	req.timer = time.AfterFunc(time.Duration(req.TimeoutSeconds)*time.Second, req.handleTimeout)
}

func (req *V2cRequest) handleTimeout() {
	req.timeoutChan <- req
}

func (req *V2cRequest) resetRetryCount() {
	req.retryCount = 0
}

func (req *V2cRequest) isRetryRequired() bool {
	req.retryCount += 1
	return req.retryCount <= req.Retries
}

func (req *V2cRequest) AddOid(oid []int32) {
	req.Varbinds = append(req.Varbinds, NewNullVarbind(oid))
}

func (req *V2cRequest) AddOids(oids [][]int32) {
	temp := make([]Varbind, len(oids))
	for i, oid := range oids {
		temp[i] = NewNullVarbind(oid)
	}
	req.Varbinds = append(req.Varbinds, temp...)
}

func (req *V2cRequest) ProcessResponse(resp SnmpResponse, err error) {
	req.responseHandler(req, resp, err)
}

type V2cResponse struct {
	v2cMessage
}

func NewV2cGetResponse(req *V2cRequest) *V2cResponse {
	resp := new(V2cResponse)
	resp.Version = Version2c
	resp.Type = GET_RESPONSE
	resp.requestId = req.requestId
	return resp
}

func NewV2cSetRequest() *V2cRequest {
	req := new(V2cRequest)
	req.Version = Version2c
	req.Type = SET_REQUEST
	return req
}
