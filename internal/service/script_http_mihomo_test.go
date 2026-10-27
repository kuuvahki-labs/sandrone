//go:build probe_mihomo

package service

func init() { nodeHTTPCompiledCores = append(nodeHTTPCompiledCores, "mihomo") }
