package gosnmp

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"sync"
	// "sync"
)

type V2cClient struct {
	snmpContext *ClientContext

	Address        *net.UDPAddr
	TimeoutSeconds int
	Retries        int
	Community      string

	mutex sync.Mutex
}

// NewV2cClient creates a new v2c client, using the default snmp port (161). It is equivalent to calling NewV2cClientWithPort(community, address, 161)
func (ctxt *ClientContext) NewV2cClient(community string, address string) (*V2cClient, error) {
	return ctxt.NewV2cClientWithPort(community, address, 161)
}

// NewV2cClientWithPort creates a new v2c client, with the initial community, host address and port as specified.
// It uses default TimeoutSeconds and Retries values of 10 and 2, meaning that by default, requests sent through this client will be sent
// 3 times, with 10 seconds in between sends, for an overall timeout of 30 seconds.
// This client is only intended to be used by a single goroutine, and as such, all calls to SendRequest() when a request is already in
// flight will cause the the calling goroutine to be blocked until all preceding calls to SendRequest are fully resolved.
func (ctxt *ClientContext) NewV2cClientWithPort(community string, address string, port int) (*V2cClient, error) {
	var err error
	client := new(V2cClient)
	client.snmpContext = ctxt
	client.Community = community
	if port < 1 || port > 65535 {
		return nil, errors.New(fmt.Sprintf("invalid port: %d", port))
	}
	address += ":" + strconv.Itoa(port)
	if client.Address, err = net.ResolveUDPAddr("udp", address); err != nil {
		return nil, err
	}
	client.TimeoutSeconds = 10
	client.Retries = 2
	return client, nil
}

// SendRequest sends one request to the host associated with this client and waits for a response or a timeout.
// The values currently set on this client for TimeoutSeconds and Retries will be used to control the request.
// On return, the request will either have a response attached, or it's error field will be filled in.
func (client *V2cClient) SendRequest(req *CommunityRequest) {
	client.mutex.Lock()
	defer client.mutex.Unlock()
	req.address = client.Address
	req.community = client.Community
	req.timeoutSeconds = client.TimeoutSeconds
	req.retriesRemaining = client.Retries
	client.snmpContext.sendRequest(req)
	req.wait()
	return
}

func (ctxt *ClientContext) AllocateV2cGetRequestWithOids(oids []ObjectIdentifier) *CommunityRequest {
	req := ctxt.AllocateV2cGetRequest()
	req.AddOids(oids)
	return req
}

func (ctxt *ClientContext) AllocateV2cGetRequest() *CommunityRequest {
	req := ctxt.allocateV2cRequest()
	req.pduType = GET_REQUEST
	return req
}

func (ctxt *ClientContext) AllocateV2cGetNextRequest() *CommunityRequest {
	req := ctxt.allocateV2cRequest()
	req.pduType = GET_NEXT_REQUEST
	return req
}

func (ctxt *ClientContext) AllocateV2cSetRequest() *CommunityRequest {
	req := ctxt.allocateV2cRequest()
	req.pduType = SET_REQUEST
	return req
}

func (ctxt *ClientContext) allocateV2cRequest() *CommunityRequest {
	req := ctxt.allocateCommunityRequest()
	req.version = Version2c
	return req
}

func (ctxt *ClientContext) FreeV2cRequest(req *CommunityRequest) {
	ctxt.freeCommunityRequest(req)
}
