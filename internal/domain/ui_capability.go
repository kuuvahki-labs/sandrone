package domain

type UICapability struct {
	Key          string   `json:"key" yaml:"key"`
	Enabled      bool     `json:"enabled" yaml:"enabled"`
	Reason       string   `json:"reason,omitempty" yaml:"reason,omitempty"`
	Dependencies []string `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`
}

type UICapabilityListResult struct {
	Features []UICapability `json:"features" yaml:"features"`
}
