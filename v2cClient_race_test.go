// +build race

package gosnmp_test

import (
	"github.com/cihub/seelog"
	snmp "github.com/idawes/gosnmp"
	handlers "github.com/idawes/gosnmp/agent_support"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
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

		Context("Sending a message to an agent", func() {
			It("should return a valid response", func(done Done) {
				agent := snmp.NewAgentWithPort("testAgent", 10, 2000, logger, new(fakeTransactionProvider))
				agent.RegisterSingleVarOidHandler(snmp.SYS_OBJECT_ID_OID, handlers.NewObjectIdentifierOidHandler(snmp.ObjectIdentifier{1, 3, 6, 1, 4, 1, 424242, 1, 1}, false))
				agent.RegisterSingleVarOidHandler(snmp.SYS_DESCR_OID, handlers.NewStringOidHandler("Test System Description", false))
				agent.SetDecodeErrorLogging(true)
				clientCtxt := snmp.NewClientContext("testClient", 1000, logger)
				clientCtxt.SetDecodeErrorLogging(true)

				numClients := 50
				clients := make([]*snmp.V2cClient, numClients)
				for i := 0; i < numClients; i++ {
					clients[i], _ = clientCtxt.NewV2cClientWithPort("private", "localhost", 2000)
				}

				retries := 2
				timeoutSeconds := 3
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
