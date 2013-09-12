package snmp_go

func NewTrapReceiver(queueDepth int, port int) (ctxt *SnmpContext, err error) {
	ctxt = new(SnmpContext)
	if err = ctxt.startReceiver(queueDepth, port); err != nil {
		return nil, err
	}
	return
}
