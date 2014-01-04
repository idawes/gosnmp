package gosnmp_test

import (
	"fmt"
	"github.com/cihub/seelog"
	"github.com/davecgh/go-spew/spew"
	snmp "github.com/idawes/gosnmp"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"math"
	"strconv"
	"sync"
	"time"
)

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
					Ω(req.GetError()).Should(BeNil())
					Ω(req.GetResponse()).Should(BeNil())
				})
			}
			AfterEach(func() {
				clientCtxt.FreeV2cRequest(req)
			})

			Context("as a GET request", func() {
				Context("with no OIDS", func() {
					BeforeEach(func() {
						req = clientCtxt.AllocateV2cGetRequest()
					})
					ValidateRequest()
				})
				Context("with a predefined set of OIDS", func() {
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
							err := req.GetError()
							Ω(err).ShouldNot(BeNil())
							_, ok := err.(snmp.TimeoutError)
							Ω(ok).Should(BeTrue())
							waitGroup.Done()
							clientCtxt.FreeV2cRequest(req)
						}(clients[i])
					}
					waitGroup.Wait()
					Ω(time.Since(start).Seconds()).Should(BeNumerically("<", float64(timeoutSeconds*(retries+1))+0.2))
					statsBin, err := clientCtxt.GetStatsBin(0)
					Ω(err).Should(BeNil())
					Ω(statsBin.Stats[snmp.StatType_RESPONSES_RECEIVED]).Should(Equal(0))
					Ω(statsBin.Stats[snmp.StatType_RESPONSES_DROPPED_BY_REQUEST_TRACKER]).Should(Equal(0))
					Ω(statsBin.Stats[snmp.StatType_UNKNOWN_REQUESTS_TIMED_OUT]).Should(Equal(0))
					Ω(statsBin.Stats[snmp.StatType_REQUESTS_TIMED_OUT]).Should(Equal(numClients * retries))
					Ω(statsBin.Stats[snmp.StatType_REQUESTS_SENT]).Should(Equal(numClients))
					Ω(statsBin.Stats[snmp.StatType_REQUEST_RETRIES_EXHAUSTED]).Should(Equal(numClients))
					Ω(statsBin.Stats[snmp.StatType_REQUESTS_FORWARDED_TO_FLOW_CONTROL]).Should(Equal(numClients * (retries + 1)))
					Ω(statsBin.Stats[snmp.StatType_OUTBOUND_MESSAGES_SENT]).Should(Equal(numClients * (retries + 1)))
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

			Context("from a single client", func() {
				Context("using 0 retries and a timeout of 1 second", func() {
					timeoutSeconds := 1
					retries := 0
					numRequests := 3
					It("should timeout after the appropriate amount of time", func(done Done) {
						var waitGroup sync.WaitGroup
						waitGroup.Add(numRequests)
						clients[0].Retries = retries
						clients[0].TimeoutSeconds = timeoutSeconds
						start := time.Now()
						for i := 0; i < numRequests; i++ {
							go func() {
								req := clientCtxt.AllocateV2cGetRequest()
								clients[0].SendRequest(req)
								err := req.GetError()
								Ω(err).ShouldNot(BeNil())
								_, ok := err.(snmp.TimeoutError)
								Ω(ok).Should(BeTrue(), spew.Sdump(req))
								waitGroup.Done()
							}()
						}
						waitGroup.Wait()
						Ω(time.Since(start).Seconds()).Should(BeNumerically("<", float64(timeoutSeconds*(retries+1)*numRequests)+0.2))
						statsBin, err := clientCtxt.GetStatsBin(0)
						Ω(err).Should(BeNil())
						Ω(statsBin.Stats[snmp.StatType_RESPONSES_RECEIVED]).Should(Equal(0))
						Ω(statsBin.Stats[snmp.StatType_RESPONSES_DROPPED_BY_REQUEST_TRACKER]).Should(Equal(0))
						Ω(statsBin.Stats[snmp.StatType_UNKNOWN_REQUESTS_TIMED_OUT]).Should(Equal(0))
						Ω(statsBin.Stats[snmp.StatType_REQUESTS_TIMED_OUT]).Should(Equal(numClients * numRequests * retries))
						Ω(statsBin.Stats[snmp.StatType_REQUESTS_SENT]).Should(Equal(numClients * numRequests))
						Ω(statsBin.Stats[snmp.StatType_REQUEST_RETRIES_EXHAUSTED]).Should(Equal(numClients * numRequests))
						Ω(statsBin.Stats[snmp.StatType_REQUESTS_FORWARDED_TO_FLOW_CONTROL]).Should(Equal(numClients * numRequests * (retries + 1)))
						Ω(statsBin.Stats[snmp.StatType_OUTBOUND_MESSAGES_SENT]).Should(Equal(numClients * numRequests * (retries + 1)))
						close(done)
					}, float64(timeoutSeconds*(retries+1)*numRequests)+2)
				})
			})
		})

		FDescribe("sending a request to an active agent", func() {
			var (
				agent *snmp.Agent
			)
			BeforeEach(func() {
				agent = snmp.NewAgentWithPort("testAgent", 10, 2000, logger)
				agent.RegisterSingleVarOidHandler(snmp.SYS_OBJECT_ID_OID, snmp.NewObjectIdentifierOidHandler(snmp.ObjectIdentifier{1, 3, 6, 1, 4, 1, 424242, 1, 1}, false))
				agent.RegisterSingleVarOidHandler(snmp.SYS_DESCR_OID, snmp.NewStringOidHandler("Test System Description", false))
				agent.SetDecodeErrorLogging(true)
				time.Sleep(1 * time.Second)
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
							err := req.GetError()
							Ω(err).Should(BeNil())
							resp := req.GetResponse()
							Ω(resp).ShouldNot(BeNil())
							logger.Debugf("Response %s", spew.Sdump(resp))
							clientCtxt.FreeV2cRequest(req)
							waitGroup.Done()
						}(clients[i])
					}
					waitGroup.Wait()
					Ω(time.Since(start).Seconds()).Should(BeNumerically("<", float64(timeoutSeconds*(retries+1))+0.2))
					statsBin, err := clientCtxt.GetStatsBin(0)
					Ω(err).Should(BeNil())
					Ω(statsBin.Stats[snmp.StatType_RESPONSES_RECEIVED]).Should(Equal(1))
					Ω(statsBin.Stats[snmp.StatType_RESPONSES_DROPPED_BY_REQUEST_TRACKER]).Should(Equal(0))
					Ω(statsBin.Stats[snmp.StatType_UNKNOWN_REQUESTS_TIMED_OUT]).Should(Equal(0))
					Ω(statsBin.Stats[snmp.StatType_REQUESTS_TIMED_OUT]).Should(Equal(numClients * retries))
					Ω(statsBin.Stats[snmp.StatType_REQUESTS_SENT]).Should(Equal(numClients))
					Ω(statsBin.Stats[snmp.StatType_REQUEST_RETRIES_EXHAUSTED]).Should(Equal(0))
					Ω(statsBin.Stats[snmp.StatType_REQUESTS_FORWARDED_TO_FLOW_CONTROL]).Should(Equal(numClients))
					Ω(statsBin.Stats[snmp.StatType_OUTBOUND_MESSAGES_SENT]).Should(Equal(numClients))
					close(done)
				}, float64(timeoutSeconds*(retries+1))+2)
			}
			Context("from a single client", func() {
				Context("using 0 retries and a timeout of 1 second", func() {
					ValidateResponse(0, 1)
				})
				// Context("using 1 retry and a timeout of 2 seconds", func() {
				// 	ValidateRequestTimeout(1, 2)
				// })
				// Context("using 2 retries and a timeout of 1 second", func() {
				// 	ValidateRequestTimeout(2, 1)
				// })
			})
		})
	})
}
