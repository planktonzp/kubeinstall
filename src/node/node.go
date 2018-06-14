package node

import "kubeinstall/src/state"

//NodeContext
type NodeContext struct {
	Context state.Context `json:"context"`
	Step    string        `json:"step"`
	HostIP  string        `json:"hostIP"`
}
