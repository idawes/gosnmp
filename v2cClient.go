package snmp_go

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"sync"
)

type V2cClient struct {
	context *SnmpContext

	// 2 channels for coordinating request/response with user of this client
	inboundRequestChan   chan *CommunityRequest
	outboundResponseChan chan *CommunityRequest
	// 1 channel for coordinating with requestTracker
	inboundReponseChan chan SnmpRequest

	Address        *net.UDPAddr
	TimeoutSeconds int
	Retries        int
	Community      string
	mutex          sync.Mutex
}

// NewV2cClient creates a new v2c client, using the default snmp port (161). It is equivalent to calling NewV2cClientWithPort(community, address, 161)
func (ctxt *SnmpContext) NewV2cClient(community string, address string) (client *V2cClient, err error) {
	return ctxt.NewV2cClientWithPort(community, address, 161)
}

// NewV2cClientWithPort creates a new v2c client, with the initial community, host address and port as specified.
// It uses default TimeoutSeconds and Retries values of 10 and 2, meaning that by default, requests sent through this client will be sent
// 3 times, with 10 seconds in between sends, for an overall timeout of 30 seconds.
// This client is only intended to be used by a single goroutine, and as such, all calls to SendRequest() when a request is already in
// flight will cause the the calling goroutine to be blocked until all preceding calls to SendRequest are fully resolved.
func (ctxt *SnmpContext) NewV2cClientWithPort(community string, address string, port int) (client *V2cClient, err error) {
	client = new(V2cClient)
	client.context = ctxt
	client.inboundRequestChan = make(chan *CommunityRequest)
	client.outboundResponseChan = make(chan *CommunityRequest)
	client.inboundReponseChan = make(chan SnmpRequest)
	client.Community = community
	if port > 65535 {
		return nil, errors.New(fmt.Sprintf("Invalid port: %d", port))
	}
	address += ":" + strconv.Itoa(port)
	if client.Address, err = net.ResolveUDPAddr("udp", address); err != nil {
		return nil, err
	}
	client.TimeoutSeconds = 10
	client.Retries = 2
	go client.processRequestChan()
	return
}

func (client *V2cClient) processRequestChan() {
	for req := range client.inboundRequestChan {
		client.processRequest(req)
		<-client.inboundReponseChan
		client.outboundResponseChan <- req
	}
}

// SendRequest sends one request to the host associated with this client and waits for a response or a timeout.
// The values currently set on this client for TimeoutSeconds and Retries will be used to control the request.
// If a response is received it will be added to the response, otherwise, the request's error field will be filled in.
func (client *V2cClient) SendRequest(req *CommunityRequest) {
	req.address = client.Address
	req.community = client.Community
	req.timeoutSeconds = client.TimeoutSeconds
	req.retries = client.Retries
	req.responseHandler = client.inboundReponseChan // specify that the response should be sent back to this client
	client.inboundRequestChan <- req                // enqueue the request. This will block if another request is already being processed by this client.
	<-client.outboundResponseChan                   // wait for a response or timeout.
	return
}

func (client *V2cClient) processRequest(req *CommunityRequest) {
	if req.responseHandler != client.inboundReponseChan {
		req.err = errors.New("Attempt to send message not created for this client")
		return
	}
	client.context.requestTracker.trackRequest(req)
	return
}

func NewV2cGetRequest() *CommunityRequest {
	req := newV2cRequest()
	req.pduType = GET_REQUEST
	return req
}

func NewV2cGetNextRequest() *CommunityRequest {
	req := newV2cRequest()
	req.pduType = GET_NEXT_REQUEST
	return req
}

func NewV2cSetRequest() *CommunityRequest {
	req := newV2cRequest()
	req.pduType = SET_REQUEST
	return req
}

func newV2cRequest() *CommunityRequest {
	req := new(CommunityRequest)
	req.version = Version2c
	return req
}
