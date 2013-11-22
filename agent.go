package gosnmp

type Agent struct {
	SnmpContext
}

func NewAgent(name string, maxTargets int, logger Logger) *Agent {
	return NewAgentWithPort(name, maxTargets, 161, logger)
}

func NewAgentWithPort(name string, maxTargets int, port int, logger Logger) *Agent {
	agent := new(Agent)
	agent.SnmpContext = *newContext(name, maxTargets, false, port, logger)
	return agent
}
