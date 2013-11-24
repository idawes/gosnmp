package gosnmp

import (
	"github.com/cihub/seelog"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"time"
)

func SetupLowLevelContextTest(logger seelog.LoggerInterface, testIdGenerator chan string) {
	Describe("Low Level SnmpContext", func() {
		var (
			clientCtxt *SnmpContext
			numClients int
			// clients    []*V2cClient
			// err        error
		)
		BeforeEach(func() {
			testId := <-testIdGenerator
			clientCtxt = NewClientContext(testId, 1000, logger)
			numClients = 1
		})
		Describe("closing the socket", func() {
			It("should cause a socket re-initialization", func() {
				time.Sleep(1 * time.Second)
				clientCtxt.conn.Close()
				time.Sleep(1 * time.Second)
				stats, err := clientCtxt.GetStatsBin(0)
				Ω(err).Should(BeNil())
				Ω(stats.Stats[INBOUND_CONNECTION_CLOSE]).Should(Equal(1))
			})
		})
	})
}
