package snmp_go

import (
	"errors"
	"fmt"
	"net"
	"strconv"
)

type V2cClient struct {
	context        *SnmpContext
	responseChan   chan *CommunityResponse
	errorChan      chan error
	Address        *net.UDPAddr
	TimeoutSeconds int
	Retries        int
	Community      string
}

func (ctxt *SnmpContext) NewV2cClient(community string, address string) (client *V2cClient, err error) {
	return ctxt.NewV2cClientWithPort(community, address, 161)
}

func (ctxt *SnmpContext) NewV2cClientWithPort(community string, address string, port int) (client *V2cClient, err error) {
	client = new(V2cClient)
	client.context = ctxt
	client.responseChan = make(chan *CommunityResponse)
	client.errorChan = make(chan error)
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
	return
}

func (client *V2cClient) SendRequest(req *CommunityRequest) (resp *CommunityResponse, err error) {
	if req.inFlight {
		return nil, errors.New("Message is already in flight")
	}
	req.inFlight = true
	req.address = client.Address
	req.community = client.Community
	req.timeoutSeconds = client.TimeoutSeconds
	req.retries = client.Retries
	req.responseHandler = client.processResponse
	client.context.requestTracker.trackRequest(req)
	select {
	case resp = <-client.responseChan:
	case err = <-client.errorChan:
	}
	req.inFlight = false
	return
}

func (client *V2cClient) processResponse(req SnmpRequest, resp SnmpResponse, err error) {
	if err != nil {
		client.errorChan <- err
	} else {
		client.responseChan <- resp.(*CommunityResponse)
	}
}

func (client *V2cClient) NewGetRequest() *CommunityRequest {
	req := new(CommunityRequest)
	req.version = Version2c
	req.pduType = GET_REQUEST
	return req
}

func (client *V2cClient) NewGetNextRequest() *CommunityRequest {
	req := new(CommunityRequest)
	req.version = Version2c
	req.pduType = GET_NEXT_REQUEST
	return req
}

func (client *V2cClient) NewSetRequest() *CommunityRequest {
	req := new(CommunityRequest)
	req.version = Version2c
	req.pduType = SET_REQUEST
	return req
}
