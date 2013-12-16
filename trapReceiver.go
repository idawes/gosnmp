package gosnmp

func NewTrapReceiver(name string, queueDepth int, port int, logger Logger) *snmpContext {
	return newContext(name, queueDepth, false, port, logger)
}
