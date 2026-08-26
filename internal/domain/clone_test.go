package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFileResultCloneOwnsMutableResultContainers(t *testing.T) {
	original := &FileResult{
		Content:  []byte("result"),
		File:     FileDocument{Content: []byte("file"), Meta: map[string]string{"source": "original"}},
		Response: ResponseInfo{Headers: map[string]string{"X-Test": "original"}},
		Report: Report{
			Warnings: []Warning{{Code: "original"}},
			Probe:    &ProbeReport{ErrorCounts: map[string]int{"timeout": 1}},
		},
	}

	cloned := original.Clone()
	cloned.Content[0] = 'R'
	cloned.File.Content[0] = 'F'
	cloned.File.Meta["source"] = "clone"
	cloned.Response.Headers["X-Test"] = "clone"
	cloned.Report.Warnings[0].Code = "clone"
	cloned.Report.Probe.ErrorCounts["timeout"] = 2

	require.Equal(t, []byte("result"), original.Content)
	require.Equal(t, []byte("file"), original.File.Content)
	require.Equal(t, "original", original.File.Meta["source"])
	require.Equal(t, "original", original.Response.Headers["X-Test"])
	require.Equal(t, "original", original.Report.Warnings[0].Code)
	require.Equal(t, 1, original.Report.Probe.ErrorCounts["timeout"])
}

func TestNodeSetClonePreservesContainerShapeAndTrafficPointers(t *testing.T) {
	total := int64(100)
	original := &NodeSet{Traffic: []SubscriptionTrafficItem{{TotalBytes: &total}}}

	cloned := original.Clone()
	*cloned.Traffic[0].TotalBytes = 50

	require.NotNil(t, cloned.Nodes)
	require.NotNil(t, cloned.Dependencies)
	require.NotNil(t, cloned.Sources)
	require.NotNil(t, cloned.Warnings)
	require.Equal(t, int64(100), *original.Traffic[0].TotalBytes)
	require.Equal(t, int64(50), *cloned.Traffic[0].TotalBytes)
	require.Nil(t, (*NodeSet)(nil).Clone())
}
