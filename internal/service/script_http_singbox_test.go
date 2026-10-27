//go:build probe_singbox

package service

func init() { nodeHTTPCompiledCores = append(nodeHTTPCompiledCores, "sing-box") }
