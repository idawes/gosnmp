package test

import (
	// "fmt"
	. "github.com/idawes/snmp_go"
	"testing"
	"time"
)

func TestAllSpecs(t *testing.T) {
	_, err := NewTrapReceiverContext(1000, 54321)
	// ctxt, err := NewSnmpClientContext(1000)
	if err != nil {
		t.Fatal(err)
	}
	// client, err := ctxt.NewV2cClientWithPort("private", "192.168.86.1", 161)
	// if err != nil {
	// 	t.Fatal(err)
	// }
	time.Sleep(time.Duration(5) * time.Minute)
	// client.TimeoutSeconds = 2
	// for i := 0; i < 2; i++ {
	// 	req := NewV2cGetRequest()
	// 	req.AddOid([]int32{1, 3, 6, 1, 4, 1, 2680, 1, 2, 7, 3, 2, 0})
	// 	resp, err := client.SendRequest(req)
	// 	if err != nil {
	// 		fmt.Println(time.Now(), err)
	// 	} else {
	// 		fmt.Printf("Got response %v for msg %v", resp, req)
	// 	}
	// }
}
