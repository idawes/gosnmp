package snmp_go

import (
	"errors"
	"fmt"
	"net"
)

type SnmpVersion int

const (
	Version1  SnmpVersion = 0x00
	Version2c             = 0x01
)

type SnmpContext struct {
	requestTracker *requestTracker
	txBufferPool   *bufferPool
	rxBufferPool   *bufferPool
	conn           *net.UDPConn
	outboundQueue  chan SnmpMessage
}

func NewSnmpClientContext(maxTargets int) (ctxt *SnmpContext, err error) {
	ctxt = new(SnmpContext)
	ctxt.requestTracker = ctxt.newRequestTracker(maxTargets)
	if ctxt.conn, err = net.ListenUDP("udp", nil); err != nil {
		return nil, errors.New(fmt.Sprintf("Couldn't bind local port, error: %s", err))
	}
	ctxt.rxBufferPool = newBufferPool(maxTargets, 2000)
	go ctxt.listen()
	ctxt.txBufferPool = newBufferPool(maxTargets*5, 64)
	ctxt.outboundQueue = make(chan SnmpMessage, maxTargets)
	go ctxt.processOutboundQueue()
	return
}

type requestTracker struct {
	context       *SnmpContext
	inboundQueue  chan SnmpMessage
	outboundQueue chan SnmpRequest
	timeoutQueue  chan SnmpRequest
	msgs          map[int32]SnmpRequest
}

func (ctxt *SnmpContext) newRequestTracker(outboundSize int) (tracker *requestTracker) {
	tracker = new(requestTracker)
	tracker.context = ctxt
	tracker.inboundQueue = make(chan SnmpMessage, 100)
	tracker.outboundQueue = make(chan SnmpRequest, outboundSize)
	tracker.timeoutQueue = make(chan SnmpRequest)
	tracker.msgs = make(map[int32]SnmpRequest)
	go tracker.startTracking()
	return
}

func (tracker *requestTracker) startTracking() {
	var nextRequestId int32 = 0
	var (
		msg SnmpMessage
		req SnmpRequest
	)
	for {
		select {
		case msg = <-tracker.inboundQueue:
			msg.getRequestId()
		case req = <-tracker.outboundQueue:
			nextRequestId += 1
			req.setRequestId(nextRequestId)
			req.resetRetryCount()
			tracker.msgs[nextRequestId] = req
			req.startTimer(tracker.timeoutQueue)
			tracker.context.outboundQueue <- req
		case req = <-tracker.timeoutQueue:
			if req.isRetryRequired() {
				req.startTimer(tracker.timeoutQueue)
				tracker.context.outboundQueue <- req
			} else {
				delete(tracker.msgs, req.getRequestId())
				req.ProcessResponse(nil, new(TimeoutError))
			}
		}
	}
}

func (tracker *requestTracker) trackRequest(req SnmpRequest) {
	tracker.outboundQueue <- req
}

func (ctxt *SnmpContext) listen() {
	for {
		buf := ctxt.rxBufferPool.getBuffer()
		b := buf.Bytes()
		b = b[:cap(b)]
		readSize, addr, err := ctxt.conn.ReadFromUDP(b)
		if err != nil {
			fmt.Printf("Couldn't read message: %s\n", err)
		} else {
			fmt.Printf("Read message of size %d from %v\n", readSize, addr)
		}
	}
}

func (ctxt *SnmpContext) processOutboundQueue() {
	for msg := range ctxt.outboundQueue {
		ctxt.conn.WriteToUDP(msg.marshal(ctxt.txBufferPool), msg.getTargetAddress())
	}
}

func (client *V2cClient) SendAsyncMsgToAddress(msg V2cMessage, address *net.UDPAddr, callback func(V2cMessage, err error)) (err error) {
	return
}

type TimeoutError struct {
}

func (t *TimeoutError) Error() string {
	return "Timed out"
}
