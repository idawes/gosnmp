// +build !race

package gosnmp_test

import (
	"fmt"
	"github.com/cihub/seelog"
	snmp "github.com/idawes/gosnmp"
	handlers "github.com/idawes/gosnmp/agent_support"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"math"
	"strconv"
	"sync"
	"time"
)

type fakeTransactionProvider struct {
}

func (provider *fakeTransactionProvider) StartTxn() interface{} {
	return 0
}

func (provider *fakeTransactionProvider) CommitTxn(interface{}) bool {
	return true
}

func (provider *fakeTransactionProvider) AbortTxn(interface{}) {
	return
}

func setupV2cClientTest(logger seelog.LoggerInterface, testIdGenerator chan string) {
	Describe("V2cClient", func() {
		var (
			clientCtxt *snmp.ClientContext
			numClients int
			clients    []*snmp.V2cClient
			err        error
		)
		BeforeEach(func() {
			testId := <-testIdGenerator
			clientCtxt = snmp.NewClientContext(testId, 1000, logger)
			clientCtxt.SetDecodeErrorLogging(true)
			numClients = 1
		})
		JustBeforeEach(func() {
			clients = make([]*snmp.V2cClient, numClients)
			for i := 0; i < numClients; i++ {
				clients[i], err = clientCtxt.NewV2cClientWithPort("private", "localhost", 2000)
			}
		})
		AfterEach(func() {
			clientCtxt.Shutdown()
		})

		Describe("creating a client", func() {
			var errClient *snmp.V2cClient
			Context("using valid parameters", func() {
				It("should create a correctly configured client", func() {
					Ω(clients[0]).ShouldNot(BeNil())
					Ω(err).Should(BeNil())
					Ω(clients[0].Community).Should(Equal("private"))
					Ω(clients[0].Address.String()).Should(Equal("127.0.0.1:" + strconv.Itoa(2000)))
				})
			})
			Context("using an invalid", func() {
				ValidateClientCreationFailure := func(errorMsg string) {
					It("should generate an error with appropriate error text", func() {
						Ω(errClient).Should(BeNil())
						Ω(err).ShouldNot(BeNil())
						Ω(err.Error()).Should(Equal(errorMsg))
					})
				}
				BeforeEach(func() {
					numClients = 0
				})
				Context("address", func() {
					BeforeEach(func() {
						errClient, err = clientCtxt.NewV2cClientWithPort("private", "blahblah", 2000)
					})
					ValidateClientCreationFailure("lookup blahblah: no such host")
				})
				Context("port (out of range low)", func() {
					BeforeEach(func() {
						errClient, err = clientCtxt.NewV2cClientWithPort("private", "localhost", 0)
					})
					ValidateClientCreationFailure("invalid port: 0")
				})
				Context("port (extremly out of range low)", func() {
					BeforeEach(func() {
						errClient, err = clientCtxt.NewV2cClientWithPort("private", "localhost", math.MinInt64)
					})
					ValidateClientCreationFailure(fmt.Sprintf("invalid port: %d", math.MinInt64))
				})
				Context("port (out of range high)", func() {
					BeforeEach(func() {
						errClient, err = clientCtxt.NewV2cClientWithPort("private", "localhost", 65536)
					})
					ValidateClientCreationFailure("invalid port: 65536")
				})
				Context("port (extremely out of range high)", func() {
					BeforeEach(func() {
						errClient, err = clientCtxt.NewV2cClientWithPort("private", "localhost", math.MaxInt64)
					})
					ValidateClientCreationFailure(fmt.Sprintf("invalid port: %d", math.MaxInt64))
				})
			})
		})

		Describe("creating a request", func() {
			var req snmp.CommunityRequest
			ValidateRequest := func() {
				It("should be possible", func() {
					Ω(req).ShouldNot(BeNil())
					Ω(req.TransportError()).Should(BeNil())
					Ω(req.Response()).Should(BeNil())
				})
			}
			AfterEach(func() {
				clientCtxt.FreeV2cRequest(req)
			})

			Context("as a GET request", func() {
				Context("with no OIDs", func() {
					BeforeEach(func() {
						req = clientCtxt.AllocateV2cGetRequest()
					})
					ValidateRequest()
				})
				Context("with a predefined set of OIDs", func() {
					BeforeEach(func() {
						oids := []snmp.ObjectIdentifier{snmp.SYS_OBJECT_ID_OID, snmp.SYS_NAME_OID, snmp.SYS_LOCATION_OID, snmp.SYS_DESCR_OID, snmp.SYS_CONTACT_OID, snmp.SYS_UPTIME_OID}
						req = clientCtxt.AllocateV2cGetRequestWithOids(oids)
					})
					ValidateRequest()
				})
			})
			Context("as a SET request", func() {
				BeforeEach(func() {
					req = clientCtxt.AllocateV2cSetRequest()
				})
				ValidateRequest()
			})
			Context("as a GETNEXT request", func() {
				BeforeEach(func() {
					req = clientCtxt.AllocateV2cGetNextRequest()
				})
				ValidateRequest()
			})
		})

		Describe("sending a request to a non-existent agent", func() {

			ValidateRequestTimeout := func(retries int, timeoutSeconds int) {
				It("should timeout after the appropriate amount of time", func(done Done) {
					var waitGroup sync.WaitGroup
					waitGroup.Add(numClients)
					start := time.Now()
					for i := 0; i < numClients; i++ {
						clients[i].Retries = retries
						clients[i].TimeoutSeconds = timeoutSeconds
						go func(client *snmp.V2cClient) {
							req := clientCtxt.AllocateV2cGetRequest()
							client.SendRequest(req)
							err := req.TransportError()
							Ω(err).ShouldNot(BeNil())
							_, ok := err.(snmp.TimeoutError)
							Ω(ok).Should(BeTrue())
							waitGroup.Done()
							clientCtxt.FreeV2cRequest(req)
						}(clients[i])
					}
					waitGroup.Wait()
					Ω(time.Since(start).Seconds()).Should(BeNumerically("<", float64(timeoutSeconds*(retries+1))+0.2))
					requestCount := numClients
					msgCount := requestCount * (retries + 1)
					validateStats(clientCtxt, map[snmp.StatType]int{
						snmp.StatType_REQUESTS_SENT:                      requestCount,
						snmp.StatType_REQUEST_RETRIES_EXHAUSTED:          requestCount,
						snmp.StatType_REQUESTS_TIMED_OUT:                 requestCount * retries,
						snmp.StatType_REQUESTS_FORWARDED_TO_FLOW_CONTROL: msgCount,
						snmp.StatType_OUTBOUND_MESSAGES_SENT:             msgCount,
					})
					close(done)
				}, float64(timeoutSeconds*(retries+1))+2)
			}

			Context("from a single client", func() {
				Context("using 0 retries and a timeout of 1 second", func() {
					ValidateRequestTimeout(0, 1)
				})
				Context("using 1 retry and a timeout of 2 seconds", func() {
					ValidateRequestTimeout(1, 2)
				})
				Context("using 2 retries and a timeout of 1 second", func() {
					ValidateRequestTimeout(2, 1)
				})
			})

			Context("from multiple clients", func() {
				BeforeEach(func() {
					numClients = 50
				})
				Context("using 0 retries and a timeout of 1 second", func() {
					ValidateRequestTimeout(0, 1)
				})
				Context("using 1 retry and a timeout of 2 seconds", func() {
					ValidateRequestTimeout(1, 2)
				})
				Context("using 2 retries and a timeout of 1 second", func() {
					ValidateRequestTimeout(2, 1)
				})
			})
		})

		Describe("sending multiple requests to a non-existent agent", func() {
			ValidateRequestTimeout := func(retries int, timeoutSeconds int, numRequests int) {
				It("should timeout after the appropriate amount of time", func(done Done) {
					var waitGroup sync.WaitGroup
					waitGroup.Add(numRequests * numClients)
					start := time.Now()
					for i := 0; i < numClients; i++ {
						clients[i].Retries = retries
						clients[i].TimeoutSeconds = timeoutSeconds
						go func(client *snmp.V2cClient) {
							for j := 0; j < numRequests; j++ {
								req := clientCtxt.AllocateV2cGetRequest()
								client.SendRequest(req)
								err := req.TransportError()
								Ω(err).ShouldNot(BeNil())
								_, ok := err.(snmp.TimeoutError)
								Ω(ok).Should(BeTrue())
								waitGroup.Done()
								clientCtxt.FreeV2cRequest(req)
							}
						}(clients[i])
					}
					waitGroup.Wait()
					Ω(time.Since(start).Seconds()).Should(BeNumerically("<", float64(timeoutSeconds*(retries+1)*numRequests)+0.2))
					requestCount := numClients * numRequests
					msgCount := requestCount * (retries + 1)
					validateStats(clientCtxt, map[snmp.StatType]int{
						snmp.StatType_REQUESTS_SENT:                      requestCount,
						snmp.StatType_REQUEST_RETRIES_EXHAUSTED:          requestCount,
						snmp.StatType_REQUESTS_TIMED_OUT:                 requestCount * retries,
						snmp.StatType_REQUESTS_FORWARDED_TO_FLOW_CONTROL: msgCount,
						snmp.StatType_OUTBOUND_MESSAGES_SENT:             msgCount,
					})
					close(done)
				}, float64(timeoutSeconds*(retries+1)*numRequests)+2)

			}
			Context("from a single client", func() {
				Context("using 0 retries and a timeout of 1 second", func() {
					ValidateRequestTimeout(0, 1, 3)
				})
				Context("using 1 retries and a timeout of 2 seconds", func() {
					ValidateRequestTimeout(1, 2, 3)
				})
				Context("using 2 retries and a timeout of 1 second", func() {
					ValidateRequestTimeout(2, 1, 3)
				})
			})
			Context("from multiple clients", func() {
				BeforeEach(func() {
					numClients = 50
				})
				Context("using 0 retries and a timeout of 1 second", func() {
					ValidateRequestTimeout(0, 1, 3)
				})
				Context("using 1 retries and a timeout of 2 seconds", func() {
					ValidateRequestTimeout(1, 2, 3)
				})
				Context("using 2 retries and a timeout of 1 second", func() {
					ValidateRequestTimeout(2, 1, 3)
				})
			})
		})

		Describe("sending a request to an active agent", func() {
			var (
				agent *snmp.Agent
			)
			BeforeEach(func() {
				agent = snmp.NewAgentWithPort("testAgent", 10, 2000, logger, new(fakeTransactionProvider))
				agent.RegisterSingleVarOidHandler(snmp.SYS_OBJECT_ID_OID, handlers.NewObjectIdentifierOidHandler(snmp.ObjectIdentifier{1, 3, 6, 1, 4, 1, 424242, 1, 1}, false))
				agent.RegisterSingleVarOidHandler(snmp.SYS_DESCR_OID, handlers.NewStringOidHandler("Test System Description", false))
				agent.SetDecodeErrorLogging(true)
			})
			AfterEach(func() {
				agent.Shutdown()
			})
			ValidateResponse := func(retries int, timeoutSeconds int) {
				It("should return a valid response", func(done Done) {
					var waitGroup sync.WaitGroup
					waitGroup.Add(numClients)
					start := time.Now()
					for i := 0; i < numClients; i++ {
						clients[i].Retries = retries
						clients[i].TimeoutSeconds = timeoutSeconds
						go func(client *snmp.V2cClient) {
							req := clientCtxt.AllocateV2cGetRequestWithOids([]snmp.ObjectIdentifier{snmp.SYS_OBJECT_ID_OID, snmp.SYS_DESCR_OID})
							client.SendRequest(req)
							err := req.TransportError()
							Ω(err).Should(BeNil())
							resp := req.Response()
							Ω(resp).ShouldNot(BeNil())

							// for _, varbind := range resp.Varbinds() {

							// }
							clientCtxt.FreeV2cRequest(req)
							waitGroup.Done()
						}(clients[i])
					}
					waitGroup.Wait()
					Ω(time.Since(start).Seconds()).Should(BeNumerically("<", float64(0.2)))
					// Check Client stats
					validateStats(clientCtxt, map[snmp.StatType]int{
						snmp.StatType_RESPONSES_RECEIVED:                 numClients,
						snmp.StatType_REQUESTS_SENT:                      numClients,
						snmp.StatType_REQUESTS_FORWARDED_TO_FLOW_CONTROL: numClients,
						snmp.StatType_OUTBOUND_MESSAGES_SENT:             numClients,
						snmp.StatType_INBOUND_MESSAGES_RECEIVED:          numClients,
						snmp.StatType_RESPONSES_RELEASED_TO_CLIENT:       numClients,
					})
					// Check Agent stats
					validateStats(agent, map[snmp.StatType]int{
						snmp.StatType_INBOUND_MESSAGES_RECEIVED: numClients,
						snmp.StatType_GET_REQUESTS_RECEIVED:     numClients,
						snmp.StatType_OUTBOUND_MESSAGES_SENT:    numClients,
					})
					close(done)
				}, 2)
			}
			Context("from a single client", func() {
				Context("using 0 retries and a timeout of 1 second", func() {
					ValidateResponse(0, 1)
				})
				Context("using 1 retry and a timeout of 2 seconds", func() {
					ValidateResponse(1, 2)
				})
				Context("using 2 retries and a timeout of 1 second", func() {
					ValidateResponse(2, 1)
				})
			})
			Context("from multiple clients", func() {
				BeforeEach(func() {
					numClients = 50
				})
				Context("using 0 retries and a timeout of 1 second", func() {
					ValidateResponse(0, 1)
				})
				Context("using 1 retry and a timeout of 2 seconds", func() {
					ValidateResponse(1, 2)
				})
				Context("using 2 retries and a timeout of 1 second", func() {
					ValidateResponse(2, 1)
				})
			})

		})
	})
}

func validateStats(provider StatsProvider, expectedValues map[snmp.StatType]int) {
	statsBin, err := provider.GetStatsBin(0)
	Ω(err).Should(BeNil())
	for statType, val := range statsBin.Stats {
		expectedVal, ok := expectedValues[statType]
		if ok {
			Ω(val).Should(Equal(expectedVal), "StatType: %s", statType)
		} else {
			Ω(val).Should(Equal(0), "StatType: %s", statType)
		}
	}
}

type StatsProvider interface {
	GetStatsBin(bin uint8) (*snmp.StatsBin, error)
}
