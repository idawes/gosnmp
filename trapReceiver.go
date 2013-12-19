package gosnmp

import (
	. "github.com/idawes/gosnmp/common"
)

func NewTrapReceiver(name string, queueDepth int, port int, logger Logger) *snmpContext {
	return newContext(name, queueDepth, false, port, logger)
}
