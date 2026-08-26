package domain

import "maps"

// Clone copies the mutable containers owned directly by a file result.
func (result *FileResult) Clone() *FileResult {
	if result == nil {
		return nil
	}
	out := *result
	out.File = result.File.Clone()
	out.Content = append([]byte{}, result.Content...)
	out.Response = result.Response.clone()
	out.Report = result.Report.Clone()
	return &out
}

// Clone copies the mutable containers owned directly by a render result.
func (result *RenderResult) Clone() *RenderResult {
	if result == nil {
		return nil
	}
	out := *result
	out.Body = append([]byte{}, result.Body...)
	out.Report = result.Report.Clone()
	return &out
}

// Clone copies the mutable containers owned by a file document.
func (doc FileDocument) Clone() FileDocument {
	out := doc
	out.Content = append([]byte{}, doc.Content...)
	out.Parts = cloneFileParts(doc.Parts)
	out.Meta = maps.Clone(doc.Meta)
	out.Warnings = append([]Warning{}, doc.Warnings...)
	return out
}

func cloneFileParts(parts []FilePart) []FilePart {
	if parts == nil {
		return nil
	}
	out := make([]FilePart, len(parts))
	for i, part := range parts {
		out[i] = part
		out[i].Content = append([]byte{}, part.Content...)
		out[i].Nodes = append([]NodeIR{}, part.Nodes...)
	}
	return out
}

// Clone copies the mutable containers owned directly by a report.
func (report Report) Clone() Report {
	out := report
	out.Dependencies = append([]ResourceRef{}, report.Dependencies...)
	out.SourceRefs = append([]SourceRef{}, report.SourceRefs...)
	out.Warnings = append([]Warning{}, report.Warnings...)
	out.Render.Warnings = append([]Warning{}, report.Render.Warnings...)
	if report.Probe != nil {
		probeReport := *report.Probe
		probeReport.ErrorCounts = maps.Clone(report.Probe.ErrorCounts)
		out.Probe = &probeReport
	}
	return out
}

// Clone copies the mutable containers owned directly by a node set.
func (set *NodeSet) Clone() *NodeSet {
	if set == nil {
		return nil
	}
	out := *set
	out.Nodes = append([]NodeIR{}, set.Nodes...)
	out.Dependencies = append([]ResourceRef{}, set.Dependencies...)
	out.Sources = append([]SourceInfo{}, set.Sources...)
	out.Warnings = append([]Warning{}, set.Warnings...)
	out.Traffic = CloneSubscriptionTrafficItems(set.Traffic)
	out.Meta = maps.Clone(set.Meta)
	return &out
}

// Clone returns an independent traffic result.
func (result *SubscriptionTrafficResult) Clone() *SubscriptionTrafficResult {
	if result == nil {
		return nil
	}
	out := *result
	out.Traffic = cloneSubscriptionTrafficItem(result.Traffic)
	return &out
}

func cloneSubscriptionTrafficItem(item *SubscriptionTrafficItem) *SubscriptionTrafficItem {
	if item == nil {
		return nil
	}
	cloned := CloneSubscriptionTrafficItems([]SubscriptionTrafficItem{*item})
	return &cloned[0]
}

// CloneSubscriptionTrafficItems copies traffic items and their optional counters.
func CloneSubscriptionTrafficItems(items []SubscriptionTrafficItem) []SubscriptionTrafficItem {
	if len(items) == 0 {
		return nil
	}
	out := make([]SubscriptionTrafficItem, len(items))
	for i, item := range items {
		out[i] = item
		out[i].TotalBytes = clonePointer(item.TotalBytes)
		out[i].RemainingBytes = clonePointer(item.RemainingBytes)
		out[i].RemainingDays = clonePointer(item.RemainingDays)
		out[i].ResetDay = clonePointer(item.ResetDay)
	}
	return out
}

func (response ResponseInfo) clone() ResponseInfo {
	response.Headers = maps.Clone(response.Headers)
	return response
}

func clonePointer[T any](value *T) *T {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
