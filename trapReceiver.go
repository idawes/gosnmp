package gosnmp

func NewTrapReceiver(name string, queueDepth int, port int, logger Logger) *SnmpContext {
	return newContext(name, queueDepth, false, port, logger)
}
