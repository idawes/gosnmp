package gosnmp_test

import (
	"fmt"
	"github.com/cihub/seelog"
	. "github.com/idawes/gosnmp"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"os"
	"testing"
)

func TestGosnmp(t *testing.T) {
	RegisterFailHandler(Fail)
	pid := os.Getpid()
	loggingConfig := fmt.Sprintf(`
<seelog>
    <outputs>
        <buffered size="10000" flushperiod="1000" formatid="debug">
            <rollingfile type="size" filename="logs/debuglog.log" maxsize="10000000" maxrolls="10" />
        </buffered>
    </outputs>
    <formats>
    	<format id="debug" format="%d %%Date(Mon Jan _2 15:04:05.000000) [%%LEV] %%File:%%Line %%Msg%%n"/>
    </formats>
</seelog>`, pid)
	logger, err := seelog.LoggerFromConfigAsBytes([]byte(loggingConfig))
	if err != nil {
		Fail(fmt.Sprintf("Couldn't initialize logger: %s", err))
	}
	testIdGenerator := make(chan string)
	go func() {
		for i := 0; ; i++ {
			testIdGenerator <- fmt.Sprintf("test %-3d", i)
		}
	}()
	setupV2cClientTest(logger, testIdGenerator)
	SetupLowLevelContextTest(logger, testIdGenerator)
	RunSpecs(t, "gosnmp Suite")
	logger.Close()
}
