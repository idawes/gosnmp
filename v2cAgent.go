package snmp_go

type V2cAgent struct {
	SnmpContext
}

func NewV2cAgent(queueDepth int) (agent *V2cAgent, err error) {
	return NewV2cAgentWithPort(queueDepth, 161)
}

func NewV2cAgentWithPort(queueDepth int, port int) (agent *V2cAgent, err error) {
	agent = new(V2cAgent)
	if err = agent.startReceiver(queueDepth, port); err != nil {
		return nil, err
	}
	return
}
