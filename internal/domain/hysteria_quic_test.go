package domain_test

import (
	"encoding/json/v2"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func TestHysteriaQUICOptionsPreserveUnknownJSONFields(t *testing.T) {
	var node domain.NodeIR
	require.NoError(t, json.Unmarshal([]byte(`{
  "name":"hy2",
  "type":"hysteria2",
  "hysteria":{
    "quic":{
      "idle_timeout":"30s",
      "future_window":1234,
      "future_flag":true
    }
  }
}`), &node))
	require.NotNil(t, node.Hysteria)
	require.NotNil(t, node.Hysteria.QUIC)
	require.Equal(t, "30s", node.Hysteria.QUIC.IdleTimeout)
	require.JSONEq(t, `1234`, string(node.Hysteria.QUIC.Unknown["future_window"]))
	require.JSONEq(t, `true`, string(node.Hysteria.QUIC.Unknown["future_flag"]))

	body, err := json.Marshal(node, json.Deterministic(true))
	require.NoError(t, err)
	require.JSONEq(t, `{
  "name":"hy2",
  "type":"hysteria2",
	"server":"",
	"port":0,
  "hysteria":{
    "quic":{
      "idle_timeout":"30s",
      "future_window":1234,
      "future_flag":true
    }
  }
}`, string(body))
}
