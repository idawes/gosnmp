package snmp_go

import (
	"errors"
	"fmt"
	. "github.com/idawes/ber_go"
	"net"
)

type SnmpVersion int

const (
	Version1  SnmpVersion = 0x00
	Version2c             = 0x01
)

func (version SnmpVersion) String() string {
	switch version {
	case Version1:
		return "SNMPv1"
	case Version2c:
		return "SNMPv2c"
	default:
		return "Unknown"
	}
}

type SnmpContext struct {
	requestTracker    *requestTracker
	berEncoderFactory *BerEncoderFactory
	conn              *net.UDPConn
	outboundQueue     chan SnmpMessage
	logger            Logger
}

func NewClientContext(maxTargets int, logger Logger) (ctxt *SnmpContext, err error) {
	ctxt = new(SnmpContext)
	ctxt.logger = logger
	ctxt.requestTracker = ctxt.newRequestTracker(maxTargets)
	if ctxt.conn, err = net.ListenUDP("udp", nil); err != nil {
		return nil, errors.New(fmt.Sprintf("Couldn't bind local port, error: %s", err))
	}
	go ctxt.listen()
	ctxt.berEncoderFactory = NewBerEncoderFactory()
	ctxt.outboundQueue = make(chan SnmpMessage, maxTargets)
	go ctxt.processOutboundQueue()
	return
}

func (ctxt *SnmpContext) startReceiver(queueDepth int, port int) (err error) {
	if ctxt.conn, err = net.ListenUDP("udp", &net.UDPAddr{Port: port}); err != nil {
		return errors.New(fmt.Sprintf("Couldn't bind local port, error: %s", err))
	}
	go ctxt.listen()
	return nil
}

type requestTracker struct {
	context       *SnmpContext
	inboundQueue  chan SnmpResponse
	outboundQueue chan SnmpRequest
	timeoutQueue  chan SnmpRequest
	msgs          map[uint32]SnmpRequest
}

func (ctxt *SnmpContext) newRequestTracker(outboundSize int) (tracker *requestTracker) {
	tracker = new(requestTracker)
	tracker.context = ctxt
	tracker.inboundQueue = make(chan SnmpResponse, 100)
	tracker.outboundQueue = make(chan SnmpRequest, outboundSize)
	tracker.timeoutQueue = make(chan SnmpRequest)
	tracker.msgs = make(map[uint32]SnmpRequest)
	go tracker.startTracking()
	return
}

func (tracker *requestTracker) startTracking() {
	var nextRequestId uint32 = 0
	var (
		resp SnmpResponse
		req  SnmpRequest
	)
	for {
		select {
		case resp = <-tracker.inboundQueue:
			req = tracker.msgs[resp.getRequestId()]
			if req == nil {
				// most likely we've already timed out the request.
				continue
			}
			req.stopTimer()
			req.setResponse(resp)
			responseHandler := req.getResponseHandler()
			if responseHandler == nil {

				responseHandler <- req
			}
		case req = <-tracker.outboundQueue:
			nextRequestId += 1
			req.setRequestId(nextRequestId)
			req.resetRetryCount()
			tracker.msgs[nextRequestId] = req
			req.startTimer(tracker.timeoutQueue)
			tracker.context.logger.Debugf("Tracker queuing outbound message %s", req.GetLoggingId())
			tracker.context.outboundQueue <- req
		case req = <-tracker.timeoutQueue:
			if req.isRetryRequired() {
				req.startTimer(tracker.timeoutQueue)
				tracker.context.outboundQueue <- req
			} else {
				delete(tracker.msgs, req.getRequestId())
				req.setError(new(TimeoutError))
				responseHandler := req.getResponseHandler()
				if responseHandler != nil {
					responseHandler <- req
				}
			}
		}
	}
}

func (tracker *requestTracker) trackRequest(req SnmpRequest) {
	tracker.outboundQueue <- req
}

func (ctxt *SnmpContext) listen() {
	msg := make([]byte, 0, 2000)
	for {
		msg = msg[0:cap(msg)]
		readLen, addr, err := ctxt.conn.ReadFromUDP(msg)
		if err != nil {
			fmt.Printf("Couldn't read message: %s\n", err)
		} else {
			ctxt.processIncomingMessage(msg[0:readLen], addr)
		}
	}
}

func (ctxt *SnmpContext) processIncomingMessage(msg []byte, addr *net.UDPAddr) {
	decodedMsg, err := decodeMsg(msg)
	if err != nil {
		fmt.Printf("Unmarshalling failed for message %v. Err: %s\n", msg, err)
		return
	}
	decodedMsg.setAddress(addr)
	fmt.Printf("Message: %#v", msg)
}

func (ctxt *SnmpContext) processOutboundQueue() {
	for msg := range ctxt.outboundQueue {
		ctxt.conn.WriteToUDP(msg.encode(ctxt.berEncoderFactory), msg.getAddress())
	}
}

type TimeoutError struct {
}

func (t TimeoutError) Error() string {
	return "Timed out"
}
