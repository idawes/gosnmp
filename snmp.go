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

type snmpContext struct {
	Logger
	logDecodeErrors bool

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

	statIncrementNotifications chan StatType
	statRequests               chan snmpContextStatRequest

	communityRequestPool *requestPool
}

func (ctxt *snmpContext) Shutdown() {
	ctxt.shutdownSync.Do(func() {
		close(ctxt.externalShutdownNotification)
		<-ctxt.shutDownComplete
	})
}

func (ctxt *snmpContext) setDecodeErrorLogging(enabled bool) {
	ctxt.logDecodeErrors = enabled
}

func newContext(name string, maxTargets int, startRequestTracker bool, port int, logger Logger) *snmpContext {
	if logger == nil {
		panic("logger must not be nil")
	}
	ctxt := new(snmpContext)
	ctxt.name = name
	ctxt.Logger = logger
	ctxt.maxTargets = maxTargets
	ctxt.port = port
	ctxt.berEncoderFactory = newberEncoderFactory(logger)
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

func (ctxt *snmpContext) monitor() {
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

type StatType int

const (
	StatType_INBOUND_CONNECTION_DEATH StatType = iota
	StatType_INBOUND_CONNECTION_CLOSE
	StatType_OUTBOUND_CONNECTION_DEATH
	StatType_OUTBOUND_CONNECTION_CLOSE
	StatType_INBOUND_MESSAGES_RECEIVED
	StatType_INBOUND_MESSAGES_UNDECODABLE
	StatType_OUTBOUND_MESSAGES_SENT
	StatType_RESPONSES_RECEIVED
	StatType_RESPONSES_RECEIVED_AFTER_REQUEST_TIMED_OUT
	StatType_REQUESTS_SENT
	StatType_REQUESTS_FORWARDED_TO_FLOW_CONTROL
	StatType_REQUESTS_TIMED_OUT_AFTER_RESPONSE_PROCESSED
	StatType_REQUESTS_TIMED_OUT
	StatType_REQUESTS_RETRIES_EXHAUSTED
	StatType_UNDECODABLE_MESSAGES_RECEIVED
	StatType_GET_REQUESTS_RECEIVED
	StatType_GET_BULK_REQUESTS_RECEIVED
	StatType_SET_REQUESTS_RECEIVED
	StatType_GET_RESPONSES_RECEIVED
)

type snmpContextStatRequest struct {
	allStats     bool
	singleStat   StatType
	bin          uint8
	responseChan chan interface{}
}

func (ctxt *snmpContext) startStatTracker() {
	ctxt.statIncrementNotifications = make(chan StatType, 100) // add some buffering to reduce likelihood of impacting throughput
	ctxt.statRequests = make(chan snmpContextStatRequest)
	go ctxt.trackStats()
}

type SnmpStatsBin struct {
	Stats      map[StatType]int
	NumSeconds int
}

func newSnmpStatsBin() *SnmpStatsBin {
	return &SnmpStatsBin{make(map[StatType]int), 0}
}

func (bin *SnmpStatsBin) copy() *SnmpStatsBin {
	binCopy := newSnmpStatsBin()
	for k, v := range bin.Stats {
		binCopy.Stats[k] = v
	}
	binCopy.NumSeconds = bin.NumSeconds
	return binCopy
}

func (ctxt *snmpContext) trackStats() {
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
			ctxt.Debugf("Ctxt %s: stats tracker shutting down due to snmpContext shutdown", ctxt.name)
			return
		}
	}
}

func (ctxt *snmpContext) incrementStat(statType StatType) {
	ctxt.statIncrementNotifications <- statType
}

func (ctxt *snmpContext) GetStat(statType StatType, bin uint8) (int, error) {
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

func (ctxt *snmpContext) GetStatsBin(bin uint8) (*SnmpStatsBin, error) {
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

func (ctxt *snmpContext) startRequestTracker(maxTargets int) {
	ctxt.requestsFromClients = make(chan SnmpRequest, maxTargets)
	ctxt.responsesFromAgents = make(chan SnmpResponse, 100)
	ctxt.requestTimeouts = make(chan uint32)
	ctxt.outstandingRequests = make(map[uint32]SnmpRequest)
	go ctxt.trackRequests()
	return
}

func (ctxt *snmpContext) sendRequest(req SnmpRequest) {
	ctxt.incrementStat(StatType_REQUESTS_SENT)
	ctxt.requestsFromClients <- req
}

func (ctxt *snmpContext) trackRequests() {
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
			ctxt.incrementStat(StatType_REQUESTS_FORWARDED_TO_FLOW_CONTROL)
			ctxt.outboundFlowControlQueue <- req

		case resp = <-ctxt.responsesFromAgents:
			req = ctxt.outstandingRequests[resp.getRequestId()]
			if req == nil {
				ctxt.incrementStat(StatType_RESPONSES_RECEIVED_AFTER_REQUEST_TIMED_OUT)
				continue // most likely we've already timed out the request.
			}
			delete(ctxt.outstandingRequests, req.getRequestId())
			req.stopTimer()
			req.setResponse(resp)
			ctxt.incrementStat(StatType_RESPONSES_RECEIVED)
			req.notify()

		case requestId := <-ctxt.requestTimeouts:
			req = ctxt.outstandingRequests[requestId]
			if req == nil {
				ctxt.incrementStat(StatType_REQUESTS_TIMED_OUT_AFTER_RESPONSE_PROCESSED)
				continue
			}
			if req.isRetryRequired() {
				req.startTimer(ctxt.handleRequestTimeout)
				ctxt.incrementStat(StatType_REQUESTS_TIMED_OUT)
				ctxt.incrementStat(StatType_REQUESTS_FORWARDED_TO_FLOW_CONTROL)
				ctxt.outboundFlowControlQueue <- req
			} else {
				delete(ctxt.outstandingRequests, req.getRequestId())
				req.setError(TimeoutError{})
				ctxt.incrementStat(StatType_REQUESTS_RETRIES_EXHAUSTED)
				ctxt.Debugf("Ctxt %s: final timeout for %s", ctxt.name, req.GetLoggingId())
				req.notify()
			}

		case <-ctxt.internalShutdownNotification:
			ctxt.Debugf("Ctxt %s: request tracker shutting down due to snmpContext shutdown", ctxt.name)
			return
		}
	}
}

func (ctxt *snmpContext) handleRequestTimeout(req SnmpRequest) {
	ctxt.requestTimeouts <- req.getRequestId()
}

// func (ctxt *snmpContext) sendResponse(resp SnmpResponse) {
// 	ctxt.outboundFlowControlQueue <- resp
// }

func (ctxt *snmpContext) processOutboundQueue() {
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
					ctxt.incrementStat(StatType_OUTBOUND_CONNECTION_CLOSE)
				} else {
					ctxt.Errorf("Ctxt %s: UDP write failed, err: %s, numWritten: %d, expected: %d", err, n, len(encodedMsg))
					ctxt.incrementStat(StatType_OUTBOUND_CONNECTION_DEATH)
				}
				return
			}
			ctxt.incrementStat(StatType_OUTBOUND_MESSAGES_SENT)
		case <-ctxt.outboundFlowControlShutdown:
			ctxt.Debugf("Ctxt %s: outbound flow controller shutting down due to shutdown message", ctxt.name)
			return
		case <-ctxt.internalShutdownNotification:
			ctxt.Debugf("Ctxt %s: outbound flow controller shutting down due to snmpContext shutdown", ctxt.name)
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

func (ctxt *snmpContext) startReceiver(port int) {
	var err error
	if ctxt.conn, err = net.ListenUDP("udp", &net.UDPAddr{Port: port}); err != nil {
		ctxt.Debugf("Ctxt %s: Couldn't bind local port: - %s", ctxt.name, err)
		ctxt.inboundDied <- true
		return
	}
	go ctxt.listen()
	return
}

func (ctxt *snmpContext) listen() {
	defer func() {
		ctxt.inboundDied <- true
		ctxt.outboundFlowControlShutdown <- true // make sure that transmit side shuts down too.
	}()
	ctxt.Debugf("Ctxt %s: incoming message listener initializing", ctxt.name)
	msg := make([]byte, 0, 2000) // UDP... 2000 bytes should be more than enough to hold the largest possible message.
	for {
		msg = msg[0:cap(msg)]
		readLen, addr, err := ctxt.conn.ReadFromUDP(msg)
		if err != nil {
			if strings.HasSuffix(err.Error(), "closed network connection") {
				ctxt.Debugf("Ctxt %s: incoming message listener shutting down", ctxt.name)
				ctxt.incrementStat(StatType_INBOUND_CONNECTION_CLOSE)
			} else {
				ctxt.Errorf("Ctxt %s: UDP read error: %#v, readLen: %d. snmpContext shutting down", ctxt.name, err, readLen)
				ctxt.incrementStat(StatType_INBOUND_CONNECTION_DEATH)
			}
			return
		} else {
			ctxt.incrementStat(StatType_INBOUND_MESSAGES_RECEIVED)
			ctxt.processIncomingMessage(msg[0:readLen], addr)
		}
	}
}

func (ctxt *snmpContext) processIncomingMessage(msg []byte, addr *net.UDPAddr) {
	decodedMsg, err := decodeMsg(msg)
	if err != nil {
		ctxt.incrementStat(StatType_UNDECODABLE_MESSAGES_RECEIVED)
		if ctxt.logDecodeErrors {
			ctxt.Debugf("Ctxt %s: Couldn't decode message % #x. Err: %s\n", ctxt.name, msg, err)
		}
		return
	}
	decodedMsg.setAddress(addr)
	switch decodedMsg.getpduType() {
	case pduType_GET_REQUEST:
		ctxt.incrementStat(StatType_GET_REQUESTS_RECEIVED)
		ctxt.processIncomingRequest(decodedMsg.(SnmpRequest))
	case pduType_GET_BULK_REQUEST:
		ctxt.incrementStat(StatType_GET_BULK_REQUESTS_RECEIVED)
		ctxt.processIncomingRequest(decodedMsg.(SnmpRequest))
	case pduType_SET_REQUEST:
		ctxt.incrementStat(StatType_SET_REQUESTS_RECEIVED)
		ctxt.processIncomingRequest(decodedMsg.(SnmpRequest))
	case pduType_GET_RESPONSE:
		ctxt.incrementStat(StatType_GET_RESPONSES_RECEIVED)
		ctxt.responsesFromAgents <- decodedMsg.(SnmpResponse)
	case pduType_V1_TRAP:
	case pduType_V2_TRAP:
	}
}

//
//
//
//
// ******************************************************************
// --------------------------- Request Pools ------------------------

func (ctxt *snmpContext) startRequestPools() {
	ctxt.communityRequestPool = newRequestPool(ctxt.maxTargets, func() SnmpRequest {
		return newCommunityRequest()
	}, ctxt)
}

func (ctxt *snmpContext) allocateCommunityRequest() *CommunityRequest {
	return ctxt.communityRequestPool.getRequest().(*CommunityRequest)
}

func (ctxt *snmpContext) freeCommunityRequest(req *CommunityRequest) {
	ctxt.communityRequestPool.putRequest(req)
}
