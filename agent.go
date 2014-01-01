package gosnmp

type Agent struct {
	snmpContext
}

func NewAgent(name string, maxTargets int, logger Logger) *Agent {
	return NewAgentWithPort(name, maxTargets, 161, logger)
}

func NewAgentWithPort(name string, maxTargets int, port int, logger Logger) *Agent {
	agent := new(Agent)
	agent.snmpContext = *newContext(name, maxTargets, false, port, logger)
	return agent
}

func (ctxt *snmpContext) processIncomingRequest(req SnmpRequest) {

}

type BasicOidHandler interface {
	Get(Varbind) error
	Set(Varbind) error
}

func (agent *Agent) register