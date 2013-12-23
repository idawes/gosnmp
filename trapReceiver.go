package gosnmp

type TrapReceiver struct {
	snmpContext
}

func NewTrapReceiver(name string, queueDepth int, port int, logger Logger) *TrapReceiver {
	trapReceiver := new(TrapReceiver)
	trapReceiver.snmpContext = *newContext(name, 0, true, 0, logger)
	return trapReceiver
}
