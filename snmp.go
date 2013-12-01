package gosnmp

import (
	"fmt"
	"github.com/davecgh/go-spew/spew"
	"math"
	"net"
	"strings"
	"sync"
	"time"
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

type snmpBlockType byte

const (
	INTEGER           snmpBlockType = 0x02
	BIT_STRING                      = 0x03
	OCTET_STRING                    = 0x04
	NULL                            = 0x05
	OBJECT_IDENTIFIER               = 0x06
	SEQUENCE                        = 0x30
	IP_ADDRESS                      = 0x40
	COUNTER_32                      = 0x41
	GAUGE_32                        = 0x42
	TIME_TICKS                      = 0x43
	OPAQUE                          = 0x44
	NSAP_ADDRESS                    = 0x45
	COUNTER_64                      = 0x46
	UINT_32                         = 0x47
)

//
//
//
//
//
// ******************************************************************
// --------------------------- Error types -------------------------

type TimeoutError struct {
}

func (t TimeoutError) Error() string {
	return "Timed out"
}

type InvalidStateError struct {
	details string
}

func (e InvalidStateError) Error() string {
	return "Invalid State: " + e.details
}

//
//
//
//
//
// ******************************************************************
// --------------------------- Context Life Cycle -------------------

type SnmpContext struct {
	Logger
	name       string
	maxTargets int
	port       int
	conn       *net.UDPConn

	// support for client request tracking
	requestsFromClients chan SnmpRequest
	responsesFromAgents chan SnmpResponse
	requestTimeouts     chan uint32
	outstandingRequests map[uint32]SnmpRequest

	//
	berEncoderFactory           *berEncoderFactory
	outboundFlowControlQueue    chan SnmpMessage
	outboundFlowControlShutdown chan bool

	shutdownSync                 sync.Once
	externalShutdownNotification chan bool
	internalShutdownNotification chan bool
	shutDownComplete             chan bool
	outboundDied                 chan bool
	inboundDied                  chan bool

	statIncrementNotifications chan SnmpContextStatType
	statRequests               chan snmpContextStatRequest

	communityRequestPool *requestPool
}

func (ctxt *SnmpContext) Shutdown() {
	ctxt.shutdownSync.Do(func() {
		close(ctxt.externalShutdownNotification)
		<-ctxt.shutDownComplete
	})
}

func NewClientContext(name string, maxTargets int, logger Logger) *SnmpContext {
	return newContext(name, maxTargets, true, 0, logger)
}

func newContext(name string, maxTargets int, startRequestTracker bool, port int, logger Logger) *SnmpContext {
	if logger == nil {
		panic("logger must not be nil")
	}
	ctxt := new(SnmpContext)
	ctxt.name = name
	ctxt.Logger = logger
	ctxt.maxTargets = maxTargets
	ctxt.port = port
	ctxt.berEncoderFactory = newBerEncoderFactory(logger)
	ctxt.outboundFlowControlQueue = make(chan SnmpMessage, ctxt.maxTargets)
	ctxt.outboundFlowControlShutdown = make(chan bool)
	ctxt.externalShutdownNotification = make(chan bool)
	ctxt.internalShutdownNotification = make(chan bool)
	ctxt.shutDownComplete = make(chan bool)
	ctxt.outboundDied = nil
	ctxt.inboundDied = nil

	ctxt.startStatTracker()
	ctxt.startRequestPools()
	if startRequestTracker {
		ctxt.startRequestTracker(maxTargets)
	}
	go ctxt.monitor()
	return ctxt
}

func (ctxt *SnmpContext) monitor() {
	shuttingDown := false
	var lastRestartAttempt time.Time
	var restartTimer <-chan time.Time
	for {
		if ctxt.outboundDied == nil && ctxt.inboundDied == nil {
			if shuttingDown {
				close(ctxt.shutDownComplete)
				ctxt.Debugf("Ctxt %s: shutdown complete", ctxt.name)
				return
			}
			restartTimerSeconds := int(math.Max(30-time.Since(lastRestartAttempt).Seconds(), 0))
			ctxt.Debugf("Ctxt %s: setting restart timer for %d seconds", ctxt.name, restartTimerSeconds)
			restartTimer = time.After(time.Duration(restartTimerSeconds) * time.Second)
		}
		select {
		case <-ctxt.externalShutdownNotification:
			ctxt.externalShutdownNotification = nil
			shuttingDown = true
			if ctxt.conn != nil {
				ctxt.conn.Close()
			}
			close(ctxt.internalShutdownNotification)
		case <-ctxt.outboundDied:
			ctxt.outboundDied = nil
		case <-ctxt.inboundDied:
			ctxt.inboundDied = nil
		case <-restartTimer:
			restartTimer = nil
			ctxt.inboundDied = make(chan bool)
			ctxt.startReceiver(ctxt.port)
			ctxt.outboundDied = make(chan bool)
			go ctxt.processOutboundQueue()
		}
	}
}

//
//
//
//
//
// *******************************************************************
// --------------------------- STATS TRACKING ------------------------

type SnmpContextStatType int

const (
	INBOUND_CONNECTION_DEATH SnmpContextStatType = iota
	INBOUND_CONNECTION_CLOSE
	OUTBOUND_CONNECTION_DEATH
	OUTBOUND_CONNECTION_CLOSE
	INBOUND_MESSAGES_RECEIVED
	INBOUND_MESSAGES_UNDECODABLE
	OUTBOUND_MESSAGES_SENT
	RESPONSES_RECEIVED
	RESPONSES_RECEIVED_AFTER_REQUEST_TIMED_OUT
	REQUESTS_SENT
	REQUESTS_FORWARDED_TO_FLOW_CONTROL
	REQUESTS_TIMED_OUT_AFTER_RESPONSE_PROCESSED
	REQUESTS_TIMED_OUT
	REQUESTS_RETRIES_EXHAUSTED
)

type snmpContextStatRequest struct {
	allStats     bool
	singleStat   SnmpContextStatType
	bin          uint8
	responseChan chan interface{}
}

func (ctxt *SnmpContext) startStatTracker() {
	ctxt.statIncrementNotifications = make(chan SnmpContextStatType, 100) // add some buffering to reduce likelihood of impacting throughput
	ctxt.statRequests = make(chan snmpContextStatRequest)
	go ctxt.trackStats()
}

type SnmpStatsBin struct {
	Stats      map[SnmpContextStatType]int
	NumSeconds int
}

func newSnmpStatsBin() *SnmpStatsBin {
	return &SnmpStatsBin{make(map[SnmpContextStatType]int), 0}
}

func (bin *SnmpStatsBin) copy() *SnmpStatsBin {
	binCopy := newSnmpStatsBin()
	for k, v := range bin.Stats {
		binCopy.Stats[k] = v
	}
	binCopy.NumSeconds = bin.NumSeconds
	return binCopy
}

func (ctxt *SnmpContext) trackStats() {
	fifteenMinuteBins := make([]*SnmpStatsBin, 97) // 96 fifteen minute bins in a day, plus one for the current bin
	fifteenMinuteBins[0] = newSnmpStatsBin()
	ticker := time.NewTicker(1 * time.Second)
	nextRollover := int(time.Now().Sub(time.Now().Truncate(15 * time.Minute)).Seconds())
	ctxt.Debugf("Ctxt %s: stats tracker initializing", ctxt.name)
	for {
		select {
		case statType := <-ctxt.statIncrementNotifications:
			fifteenMinuteBins[0].Stats[statType] += 1

		case req := <-ctxt.statRequests:
			ctxt.Debugf("Ctxt %s: got stats request", ctxt.name)
			if req.bin >= uint8(len(fifteenMinuteBins)) {
				req.responseChan <- nil
			}
			statsBin := fifteenMinuteBins[req.bin]
			if statsBin.Stats == nil {
				req.responseChan <- nil
			}
			if req.allStats {
				req.responseChan <- statsBin.copy()
			} else {
				req.responseChan <- statsBin.Stats[req.singleStat]
			}

		case <-ticker.C:
			fifteenMinuteBins[0].NumSeconds++
			if fifteenMinuteBins[0].NumSeconds == nextRollover {
				for idx := len(fifteenMinuteBins); idx > 0; idx-- {
					fifteenMinuteBins[idx] = fifteenMinuteBins[idx-1]
				}
				fifteenMinuteBins[0] = newSnmpStatsBin()
				nextRollover = int(15 * time.Minute.Seconds())
			}

		case <-ctxt.internalShutdownNotification:
			ticker.Stop()
			ctxt.Debugf("Ctxt %s: stats tracker shutting down due to SnmpContext shutdown", ctxt.name)
			return
		}
	}
}

func (ctxt *SnmpContext) incrementStat(statType SnmpContextStatType) {
	ctxt.statIncrementNotifications <- statType
}

func (ctxt *SnmpContext) GetStat(statType SnmpContextStatType, bin uint8) (int, error) {
	responseChan := make(chan interface{})
	ctxt.statRequests <- snmpContextStatRequest{singleStat: statType, bin: bin, responseChan: responseChan}
	resp := <-responseChan
	if resp == nil {
		return 0, fmt.Errorf("The requested bin (%d) is not available", bin)
	}
	statVal, ok := resp.(int)
	if !ok {
		ctxt.Errorf("Couldn't cast response %#v to int", resp)
		return 0, fmt.Errorf("Internal error, couldn't retrieve stat")
	}
	return statVal, nil
}

func (ctxt *SnmpContext) GetStatsBin(bin uint8) (*SnmpStatsBin, error) {
	responseChan := make(chan interface{})
	ctxt.statRequests <- snmpContextStatRequest{allStats: true, bin: bin, responseChan: responseChan}
	resp := <-responseChan
	if resp == nil {
		return nil, fmt.Errorf("The requested bin (%d) is not available", bin)
	}
	stats, ok := resp.(*SnmpStatsBin)
	if !ok {
		ctxt.Errorf("Couldn't cast response %#v to map", resp)
		return nil, fmt.Errorf("Internal error, couldn't retrieve stat")
	}
	return stats, nil
}

//
//
//
//
//
// *******************************************************************
// --------------------------- TRANSMIT SIDE -------------------------

func (ctxt *SnmpContext) startRequestTracker(maxTargets int) {
	ctxt.requestsFromClients = make(chan SnmpRequest, maxTargets)
	ctxt.responsesFromAgents = make(chan SnmpResponse, 100)
	ctxt.requestTimeouts = make(chan uint32)
	ctxt.outstandingRequests = make(map[uint32]SnmpRequest)
	go ctxt.trackRequests()
	return
}

func (ctxt *SnmpContext) sendRequest(req SnmpRequest) {
	ctxt.incrementStat(REQUESTS_SENT)
	ctxt.requestsFromClients <- req
}

func (ctxt *SnmpContext) trackRequests() {
	var nextRequestId uint32 = 0
	var (
		resp SnmpResponse
		req  SnmpRequest
	)
	ctxt.Debugf("Ctxt %s: request tracker initializing", ctxt.name)
	for {
		select {
		case req = <-ctxt.requestsFromClients:
			nextRequestId += 1
			req.setRequestId(nextRequestId)
			ctxt.outstandingRequests[nextRequestId] = req
			req.startTimer(ctxt.handleRequestTimeout)
			ctxt.incrementStat(REQUESTS_FORWARDED_TO_FLOW_CONTROL)
			ctxt.outboundFlowControlQueue <- req

		case resp = <-ctxt.responsesFromAgents:
			req = ctxt.outstandingRequests[resp.getRequestId()]
			if req == nil {
				ctxt.incrementStat(RESPONSES_RECEIVED_AFTER_REQUEST_TIMED_OUT)
				continue // most likely we've already timed out the request.
			}
			delete(ctxt.outstandingRequests, req.getRequestId())
			req.stopTimer()
			req.setResponse(resp)
			ctxt.incrementStat(RESPONSES_RECEIVED)
			req.notify()

		case requestId := <-ctxt.requestTimeouts:
			req = ctxt.outstandingRequests[requestId]
			if req == nil {
				ctxt.incrementStat(REQUESTS_TIMED_OUT_AFTER_RESPONSE_PROCESSED)
				continue
			}
			if req.isRetryRequired() {
				req.startTimer(ctxt.handleRequestTimeout)
				ctxt.incrementStat(REQUESTS_TIMED_OUT)
				ctxt.incrementStat(REQUESTS_FORWARDED_TO_FLOW_CONTROL)
				ctxt.outboundFlowControlQueue <- req
			} else {
				delete(ctxt.outstandingRequests, req.getRequestId())
				req.setError(TimeoutError{})
				ctxt.incrementStat(REQUESTS_RETRIES_EXHAUSTED)
				ctxt.Debugf("Ctxt %s: final timeout for %s", ctxt.name, req.GetLoggingId())
				req.notify()
			}

		case <-ctxt.internalShutdownNotification:
			ctxt.Debugf("Ctxt %s: request tracker shutting down due to SnmpContext shutdown", ctxt.name)
			return
		}
	}
}

func (ctxt *SnmpContext) handleRequestTimeout(req SnmpRequest) {
	ctxt.requestTimeouts <- req.getRequestId()
}

// func (ctxt *SnmpContext) sendResponse(resp SnmpResponse) {
// 	ctxt.outboundFlowControlQueue <- resp
// }

func (ctxt *SnmpContext) processOutboundQueue() {
	defer func() {
		ctxt.outboundDied <- true
		ctxt.conn.Close() // make sure that receive side shuts down too.
	}()
	ctxt.Debugf("Ctxt %s: outbound flow controller initializing", ctxt.name)
	for {
		select {
		case msg := <-ctxt.outboundFlowControlQueue:
			encodedMsg, err := msg.encode(ctxt.berEncoderFactory)
			if err != nil {
				ctxt.Debugf("Couldn't encode message: err: %s. Message:\n%s", err, spew.Sdump(msg))
				continue
			}
			if n, err := ctxt.conn.WriteToUDP(encodedMsg, msg.getAddress()); err != nil || n != len(encodedMsg) {
				if strings.HasSuffix(err.Error(), "closed network connection") {
					ctxt.Debugf("Ctxt %s: outbound flow controller shutting down due to closed connection", ctxt.name)
					ctxt.incrementStat(OUTBOUND_CONNECTION_CLOSE)
				} else {
					ctxt.Errorf("Ctxt %s: UDP write failed, err: %s, numWritten: %d, expected: %d", err, n, len(encodedMsg))
					ctxt.incrementStat(OUTBOUND_CONNECTION_DEATH)
				}
				return
			}
			ctxt.incrementStat(OUTBOUND_MESSAGES_SENT)
		case <-ctxt.outboundFlowControlShutdown:
			ctxt.Debugf("Ctxt %s: outbound flow controller shutting down due to shutdown message", ctxt.name)
			return
		case <-ctxt.internalShutdownNotification:
			ctxt.Debugf("Ctxt %s: outbound flow controller shutting down due to SnmpContext shutdown", ctxt.name)
			return
		}
	}
}

//
//
//
//
// ******************************************************************
// --------------------------- RECEIVE SIDE -------------------------

func (ctxt *SnmpContext) startReceiver(port int) {
	var err error
	if ctxt.conn, err = net.ListenUDP("udp", &net.UDPAddr{Port: port}); err != nil {
		ctxt.Debugf("Ctxt %s: Couldn't bind local port: - %s", ctxt.name, err)
		ctxt.inboundDied <- true
		return
	}
	go ctxt.listen()
	return
}

func (ctxt *SnmpContext) listen() {
	defer func() {
		ctxt.inboundDied <- true
		ctxt.outboundFlowControlShutdown <- true // make sure that transmit side shuts down too.
	}()
	ctxt.Debugf("Ctxt %s: incoming message listener initializing", ctxt.name)
	msg := make([]byte, 0, 2000)
	for {
		msg = msg[0:cap(msg)]
		readLen, addr, err := ctxt.conn.ReadFromUDP(msg)
		if err != nil {
			if strings.HasSuffix(err.Error(), "closed network connection") {
				ctxt.Debugf("Ctxt %s: incoming message listener shutting down", ctxt.name)
				ctxt.incrementStat(INBOUND_CONNECTION_CLOSE)
			} else {
				ctxt.Errorf("Ctxt %s: UDP read error: %#v, readLen: %d. SnmpContext shutting down", ctxt.name, err, readLen)
				ctxt.incrementStat(INBOUND_CONNECTION_DEATH)
			}
			return
		} else {
			ctxt.incrementStat(INBOUND_MESSAGES_RECEIVED)
			ctxt.processIncomingMessage(msg[0:readLen], addr)
		}
	}
}

func (ctxt *SnmpContext) processIncomingMessage(msg []byte, addr *net.UDPAddr) {
	decodedMsg, err := decodeMsg(msg)
	if err != nil {
		ctxt.Debugf("Ctxt %s: Couldn't decode message % #x. Err: %s\n", ctxt.name, msg, err)
		return
	}
	decodedMsg.setAddress(addr)
}

//
//
//
//
// ******************************************************************
// --------------------------- Request Pools ------------------------

func (ctxt *SnmpContext) startRequestPools() {
	ctxt.communityRequestPool = newRequestPool(ctxt.maxTargets, func() SnmpRequest {
		return newCommunityRequest()
	}, ctxt)
}

func (ctxt *SnmpContext) allocateCommunityRequest() *CommunityRequest {
	return ctxt.communityRequestPool.getRequest().(*CommunityRequest)
}

func (ctxt *SnmpContext) freeCommunityRequest(req *CommunityRequest) {
	ctxt.communityRequestPool.putRequest(req)
}
