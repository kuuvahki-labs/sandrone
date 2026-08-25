package domain

// SetNodeLineage attaches runtime-only lineage to a node. Lineage is not part
// of canonical NodeIR data and is never serialized.
func SetNodeLineage(node *NodeIR, lineage string) {
	if node == nil {
		return
	}
	node.lineage = lineage
}

// NodeLineage returns runtime-only lineage attached by an orchestrating flow.
func NodeLineage(node NodeIR) string {
	return node.lineage
}

// ClearNodeLineage removes runtime-only lineage before a node leaves the flow
// that owns it.
func ClearNodeLineage(node *NodeIR) {
	SetNodeLineage(node, "")
}
