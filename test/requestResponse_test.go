package test

import (
	"fmt"
	"github.com/cihub/seelog"
	. "github.com/idawes/snmp_go"
	. "launchpad.net/gocheck"
	"math"
	"strconv"
	"time"
)

type RequestResponseTestSuite struct {
	clientCtxt *SnmpContext
	// agent      *V2cAgent
	logger Logger
}

func NewRequestResponseTestSuite() (s *RequestResponseTestSuite, err error) {
	s = new(RequestResponseTestSuite)
	loggingConfig := `
<seelog>
    <outputs>
        <buffered size="10000" flushperiod="1000" formatid="debug">
            <rollingfile type="size" filename="logs/debuglog" maxsize="10000000" maxrolls="10" />
        </buffered>
    </outputs>
    <formats>
    	<format id="debug" format="%Date(Mon Jan _2 15:04:05.000000) [%LEV] %File:%Line %Msg%n"/>
    </formats>
</seelog>`
	s.logger, err = seelog.LoggerFromConfigAsBytes([]byte(loggingConfig))
	if err != nil {
		err = fmt.Errorf("Couldn't initialize logger: %s", err)
		return
	}
	s.clientCtxt, err = NewClientContext(1000, s.logger)
	return
}

func (s *RequestResponseTestSuite) TestSimpleTimeout(c *C) {
	c.Log("Given a client context, check that it is possible to create a client pointing at a non-existent agent")
	client := s.createSimpleCLient(c, 2000)
	c.Log("  Then check that is is possible to create a request and run it using that client")
	req := NewV2cGetRequest()
	c.Assert(req, NotNil)
	req.AddOid(SYS_DESCR_OID)
	client.SendRequest(req)
	checkTimeout(c, client, req, 1)
}

func (s *RequestResponseTestSuite) TestSequentialClientAccess(c *C) {
	c.Log("Given a client context, check that it is possible to create a client pointing at a non-existent agent")
	client := s.createSimpleCLient(c, 2000)
	c.Log("  Then check that is is possible to create a request and run it using that client")
	startTime := time.Now()
	req1 := NewV2cGetRequest()
	c.Assert(req1, NotNil)
	req1.AddOid(SYS_DESCR_OID)
	req2 := NewV2cGetRequest()
	c.Assert(req2, NotNil)
	req2.AddOid(SYS_DESCR_OID)
	done := make(chan bool, 2)
	go sendRequest(client, req1, done)
	go sendRequest(client, req2, done)
	<-done
	<-done
	checkTimeout(c, client, req1, 1)
	checkTimeout(c, client, req2, 2)
	c.Log("  Also, because both requests were executed on the same client, they should have been executed sequentially ")
	c.Check(math.Abs(time.Since(startTime).Seconds()-float64(2*((client.Retries+1)*client.TimeoutSeconds))) < 0.1, Equals, true, Commentf("Requests took %v", time.Since(startTime)))
}

func (s *RequestResponseTestSuite) TestConcurrentTimeouts(c *C) {
	c.Log("Given a client context, check that it is possible to create a number of clients pointing at non-existent agents")
	numClients := 500
	done := make(chan bool, numClients)
	clients := make([]*V2cClient, 0, numClients)
	for i := 0; i < numClients; i++ {
		clients = append(clients, s.createSimpleCLient(c, 2000))
	}
	c.Log("  Then check that it is possible to create and run requests concurrently to all of the clients")
	startTime := time.Now()
	requests := make([]*CommunityRequest, 0, numClients)
	for i := 0; i < numClients; i++ {
		req := NewV2cGetRequest()
		c.Assert(req, NotNil)
		req.AddOid(SYS_DESCR_OID)
		requests = append(requests, req)
	}
	for i := 0; i < numClients; i++ {
		go sendRequest(clients[i], requests[i], done)
	}
	for i := 0; i < numClients; i++ {
		<-done
	}
	c.Log("    Each of which should fail due to a timeout, and the flightTime of each request should be close to the correct timeout value")
	for i := 0; i < numClients; i++ {
		checkTimeout(c, clients[i], requests[i], i)
	}
	c.Log("    And, because we're running concurrently, the whole test should take approximately the same amount of time as a single request timeout ")
	c.Check(math.Abs(time.Since(startTime).Seconds()-float64(((clients[0].Retries+1)*clients[0].TimeoutSeconds))) < 0.1, Equals, true, Commentf("Requests took %v", time.Since(startTime)))
	c.Log("  Now, change each client's timeout and re-send the requests.")
	startTime = time.Now()
	for _, client := range clients {
		client.TimeoutSeconds = 2
	}
	for i := 0; i < numClients; i++ {
		go sendRequest(clients[i], requests[i], done)
	}
	for i := 0; i < numClients; i++ {
		<-done
	}
	c.Log("    Each of which should fail due to a timeout, and the flightTime of each request should be close to the correct timeout value")
	for i := 0; i < numClients; i++ {
		checkTimeout(c, clients[i], requests[i], i+1)
	}
	c.Log("    And, because we're running concurrently, the whole test should take approximately the same amount of time as a single request timeout ")
	c.Check(math.Abs(time.Since(startTime).Seconds()-float64(((clients[0].Retries+1)*clients[0].TimeoutSeconds))) < 0.1, Equals, true, Commentf("Requests took %v", time.Since(startTime)))
}

func (s *RequestResponseTestSuite) createSimpleCLient(c *C, port int) (client *V2cClient) {
	client, err := s.clientCtxt.NewV2cClientWithPort("private", "localhost", port)
	c.Assert(client, NotNil)
	c.Assert(err, IsNil)
	c.Assert(client.Community, Equals, "private")
	c.Assert(client.Address.String(), Equals, "127.0.0.1:"+strconv.Itoa(port))
	client.TimeoutSeconds = 1
	client.Retries = 0
	return
}

func sendRequest(client *V2cClient, req *CommunityRequest, done chan bool) {
	defer func() { done <- true }()
	client.SendRequest(req)
	return
}

func checkTimeout(c *C, client *V2cClient, req *CommunityRequest, requestId int) {
	resp := req.GetResponse()
	err := req.GetError()
	c.Assert(resp, IsNil)
	c.Assert(err, NotNil)
	_, ok := err.(*TimeoutError)
	c.Assert(ok, Equals, true)
	c.Check(math.Abs(req.GetFlightTime().Seconds()-float64(((client.Retries+1)*client.TimeoutSeconds))) < 0.01, Equals, true, Commentf("Request %d took %v", requestId, req.GetFlightTime()))
}
