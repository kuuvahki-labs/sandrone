package domain

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"maps"
)

type hysteriaQUICWire HysteriaQUICOptions

var hysteriaQUICKnownFields = map[string]bool{
	"idle_timeout":                      true,
	"keep_alive_period":                 true,
	"stream_receive_window":             true,
	"connection_receive_window":         true,
	"max_concurrent_streams":            true,
	"initial_packet_size":               true,
	"disable_path_mtu_discovery":        true,
	"initial_stream_receive_window":     true,
	"max_stream_receive_window":         true,
	"initial_connection_receive_window": true,
	"max_connection_receive_window":     true,
}

func (o *HysteriaQUICOptions) UnmarshalJSON(data []byte) error {
	var wire hysteriaQUICWire
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	var raw map[string]jsontext.Value
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	maps.DeleteFunc(raw, func(key string, _ jsontext.Value) bool {
		return hysteriaQUICKnownFields[key]
	})
	*o = HysteriaQUICOptions(wire)
	if len(raw) > 0 {
		o.Unknown = raw
	}
	return nil
}

func (o HysteriaQUICOptions) MarshalJSON() ([]byte, error) {
	wireBody, err := json.Marshal(hysteriaQUICWire(o), json.Deterministic(true))
	if err != nil {
		return nil, err
	}
	var wire map[string]jsontext.Value
	if err := json.Unmarshal(wireBody, &wire); err != nil {
		return nil, err
	}
	for key, value := range o.Unknown {
		if !hysteriaQUICKnownFields[key] {
			wire[key] = value
		}
	}
	return json.Marshal(wire, json.Deterministic(true))
}

func (o HysteriaQUICOptions) IsZero() bool {
	return o.IdleTimeout == "" &&
		o.KeepAlivePeriod == "" &&
		o.StreamReceiveWindow == 0 &&
		o.ConnectionReceiveWindow == 0 &&
		o.MaxConcurrentStreams == 0 &&
		o.InitialPacketSize == 0 &&
		!o.DisablePathMTUDiscovery &&
		o.InitialStreamReceiveWindow == 0 &&
		o.MaxStreamReceiveWindow == 0 &&
		o.InitialConnectionReceiveWindow == 0 &&
		o.MaxConnectionReceiveWindow == 0 &&
		len(o.Unknown) == 0
}
