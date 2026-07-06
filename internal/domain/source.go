package domain

type SourceRef struct {
	Kind     string `json:"kind" yaml:"kind"`
	Name     string `json:"name" yaml:"name"`
	URL      string `json:"url,omitempty" yaml:"url,omitempty"`
	Repo     string `json:"repo,omitempty" yaml:"repo,omitempty"`
	Revision string `json:"revision,omitempty" yaml:"revision,omitempty"`
	Path     string `json:"path,omitempty" yaml:"path,omitempty"`
	Lines    string `json:"lines,omitempty" yaml:"lines,omitempty"`
	Note     string `json:"note,omitempty" yaml:"note,omitempty"`
}

type SourceInfo struct {
	Format     string      `json:"format" yaml:"format"`
	SourceRefs []SourceRef `json:"source_refs,omitempty" yaml:"source_refs,omitempty"`
	Warnings   []Warning   `json:"warnings,omitempty" yaml:"warnings,omitempty"`
}
