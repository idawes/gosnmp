package gosnmp

import ()

type requestPool struct {
	freeList         chan SnmpRequest
	createNewRequest func() SnmpRequest
	logger           Logger
}

// newRequestPool creates an empty request pool with space to hold n requests
func newRequestPool(n int, createNewRequest func() SnmpRequest, logger Logger) *requestPool {
	pool := new(requestPool)
	pool.freeList = make(chan SnmpRequest, n)
	pool.createNewRequest = createNewRequest
	pool.logger = logger
	return pool
}

// getRequest gets a request from the pool. If the pool is empty, a new request will be allocated
func (p *requestPool) getRequest() SnmpRequest {
	var req SnmpRequest
	select {
	case req = <-p.freeList:
		// we got a request from the free list
	default:
		// free list doesn't have a request for us... create a new one.
		req = p.createNewRequest()
	}
	return req
}

// putRequest returns a request to the pool. If the pool is full, the request will be garbage collected
func (p *requestPool) putRequest(req SnmpRequest) {
	select {
	case p.freeList <- req:
		// request is back on the freelist
	default:
		// freeList is full... request will be gc'd
		p.logger.Debugf("Trashing snmp request - community request pool may not be correctly sized")
	}
}
