package domain

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"github.com/gofrs/uuid/v5"
)

// NodeIdentity exposes the two identity dimensions of a normalized node.
// RuntimeID follows one node occurrence through a materialization pipeline;
// ConnectionKey identifies its current canonical connection semantics.
type NodeIdentity struct {
	RuntimeID     string
	ConnectionKey string
}

// AssignNodeRuntimeIDs ensures every node occurrence has a unique runtime ID.
// Existing unique IDs survive processor passes; missing or copied IDs are new.
func AssignNodeRuntimeIDs(nodes []NodeIR) error {
	seen := make(map[string]struct{}, len(nodes))
	for index := range nodes {
		id := nodes[index].runtimeID
		if id != "" {
			if _, exists := seen[id]; !exists {
				seen[id] = struct{}{}
				continue
			}
		}
		for {
			generated, err := uuid.NewV4()
			if err != nil {
				return fmt.Errorf("generate runtime ID for node %d: %w", index, err)
			}
			id = generated.String()
			if _, exists := seen[id]; !exists {
				break
			}
		}
		nodes[index].runtimeID = id
		seen[id] = struct{}{}
	}
	return nil
}

// NodeRuntimeID returns the runtime-only occurrence ID attached to a node.
func NodeRuntimeID(node NodeIR) string {
	return node.runtimeID
}

// SetNodeRuntimeID restores runtime identity across controlled conversions,
// such as the private Goja script envelope. It must not be exposed to scripts.
func SetNodeRuntimeID(node *NodeIR, id string) {
	if node != nil {
		node.runtimeID = id
	}
}

// ClearNodeRuntimeID strips runtime identity before a node is exposed as data.
func ClearNodeRuntimeID(node *NodeIR) {
	SetNodeRuntimeID(node, "")
}

// NodeConnectionKey returns a content key for canonical connection
// semantics. Display, processor metadata and diagnostics do not affect it.
func NodeConnectionKey(node NodeIR) (string, error) {
	node.Name = ""
	node.Tags = nil
	node.Meta = nil
	node.Raw = nil
	node.Lossy = false
	node.Warnings = nil
	node.SourceFormat = ""
	body, err := json.Marshal(node) //nolint:gosec // Credentials are hashed immediately and never returned.
	if err != nil {
		return "", fmt.Errorf("encode node connection semantics: %w", err)
	}
	sum := sha256.Sum256(body)
	return fmt.Sprintf("%x", sum[:]), nil
}

// IdentifyNode returns both identity dimensions from the node's current state.
func IdentifyNode(node NodeIR) (NodeIdentity, error) {
	key, err := NodeConnectionKey(node)
	if err != nil {
		return NodeIdentity{}, err
	}
	return NodeIdentity{RuntimeID: node.runtimeID, ConnectionKey: key}, nil
}
