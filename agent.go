package snmp_go

type Agent struct {
	SnmpContext
}

func NewAgent(queueDepth int) (agent *Agent, err error) {
	return NewAgentWithPort(queueDepth, 161)
}

func NewAgentWithPort(queueDepth int, port int) (agent *Agent, err error) {
	agent = new(Agent)
	if err = agent.startReceiver(queueDepth, port); err != nil {
		return nil, err
	}
	return
}
