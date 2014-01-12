package gosnmp

type ClientContext struct {
	snmpContext
}

func NewClientContext(name string, maxTargets int, logger Logger) *ClientContext {
	client := new(ClientContext)
	client.snmpContext.initContext(name, maxTargets, true, 0, logger)
	return client
}
