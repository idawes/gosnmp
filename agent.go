package gosnmp

import (
	"code.google.com/p/biogo.llrb"
)

type Agent struct {
	snmpContext
	oidTree llrb.Tree
}

func NewAgent(name string, maxTargets int, logger Logger) *Agent {
	return NewAgentWithPort(name, maxTargets, 161, logger)
}

func NewAgentWithPort(name string, maxTargets int, port int, logger Logger) *Agent {
	agent := new(Agent)
	agent.snmpContext.initContext(name, maxTargets, false, port, logger)
	agent.incomingRequestProcessor = agent
	agent.oidTree = llrb.Tree{}
	return agent
}

func (agent *Agent) processcommunityRequest(req *communityRequest) {
	resp := req.createResponse()
	for _, requestVb := range req.varbinds {
		node := agent.lookupHandler(requestVb.getOid())
		if node == nil {
			resp.AddVarbind(NewNoSuchObjectVarbind(requestVb.getOid()))
			continue
		}
		switch req.GetRequestType() {
		case pduType_GET_REQUEST:
			responseVb, err := node.handler.Get(requestVb.getOid())
			if err != nil {
				continue
			}
			resp.AddVarbind(responseVb)

		case pduType_SET_REQUEST:

		}
	}
	agent.sendResponse(resp)
}

func (agent *Agent) lookupHandler(oid ObjectIdentifier) *oidTreeNode {
	var node *oidTreeNode
	matchLength := 0
	agent.oidTree.Do(func(n llrb.Comparable) (done bool) {
		testNode := n.(*oidTreeNode)
		if testMatchLength := testNode.oid.findMatchLength(oid); testMatchLength > matchLength {
			matchLength = testMatchLength
			node = testNode
		}
		return
	})
	return node
}

type oidHandler interface {
	Get(ObjectIdentifier) (Varbind, error)
	Set(Varbind) error
}

type SingleVarOidHandler interface {
	oidHandler
}

func (agent *Agent) RegisterSingleVarOidHandler(oid ObjectIdentifier, handler SingleVarOidHandler) error {
	agent.oidTree.Insert(&oidTreeNode{oid, false, handler})
	return nil
}

type oidTreeNode struct {
	oid     ObjectIdentifier
	isMulti bool
	handler oidHandler
}

func (a *oidTreeNode) Compare(b llrb.Comparable) int {
	return a.oid.Compare(b.(*oidTreeNode).oid)
}

type oidTreeLookup ObjectIdentifier

func (a oidTreeLookup) Compare(b llrb.Comparable) int {
	return ObjectIdentifier(a).Compare(b.(*oidTreeNode).oid)
}
