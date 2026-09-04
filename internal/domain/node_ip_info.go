package domain

type NodeInspectField string

const (
	NodeInspectURI NodeInspectField = "uri"
	NodeInspectIP  NodeInspectField = "ip"
)

type NodeInspectRequest struct {
	Node    NodeIR             `json:"node" yaml:"node"`
	Include []NodeInspectField `json:"include" yaml:"include"`
}

type NodeURIInfo struct {
	Value    string    `json:"value" yaml:"value"`
	Warnings []Warning `json:"warnings,omitempty" yaml:"warnings,omitempty"`
}

type NodeInspectResult struct {
	URI *NodeURIInfo      `json:"uri,omitempty" yaml:"uri,omitempty"`
	IP  *NodeIPInfoResult `json:"ip,omitempty" yaml:"ip,omitempty"`
}

type NodeIPInfoSource struct {
	Name string `json:"name" yaml:"name"`
	URL  string `json:"url" yaml:"url"`
}

type NodeIPInfoResult struct {
	Server        string            `json:"server" yaml:"server"`
	IP            string            `json:"ip" yaml:"ip"`
	IPVersion     int               `json:"ip_version" yaml:"ip_version"`
	Public        bool              `json:"public" yaml:"public"`
	CountryCode   string            `json:"country_code,omitempty" yaml:"country_code,omitempty"`
	Country       string            `json:"country,omitempty" yaml:"country,omitempty"`
	ContinentCode string            `json:"continent_code,omitempty" yaml:"continent_code,omitempty"`
	Continent     string            `json:"continent,omitempty" yaml:"continent,omitempty"`
	ASN           string            `json:"asn,omitempty" yaml:"asn,omitempty"`
	ASName        string            `json:"as_name,omitempty" yaml:"as_name,omitempty"`
	ASDomain      string            `json:"as_domain,omitempty" yaml:"as_domain,omitempty"`
	Source        *NodeIPInfoSource `json:"source,omitempty" yaml:"source,omitempty"`
}
