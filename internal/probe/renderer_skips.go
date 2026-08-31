//go:build probe_mihomo || probe_singbox

package probe

func rendererSkippedNodeMessages(payload *Payload, target string, nodeCount int) map[int]string {
	if payload == nil {
		return nil
	}
	messages := map[int]string{}
	for _, warning := range payload.RenderReport.Warnings {
		if warning.Code != "render_node_skipped" || warning.Target != target || warning.NodeIndex == nil {
			continue
		}
		index := *warning.NodeIndex
		if index < 0 || index >= nodeCount {
			continue
		}
		messages[index] = warning.Message
	}
	return messages
}
