package script_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/processor"
	"github.com/kuuvahki-labs/sandrone/internal/processor/script"
)

func exitInfoSource(t *testing.T) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("..", "..", "..", "examples", "scripts", "exit-info.js"))
	require.NoError(t, err)
	return string(body)
}

func TestExampleExitInfoOneGETValidationAndStableNames(t *testing.T) {
	calls := 0
	registry := processor.NewRegistry()
	registerScript(registry, script.WithHTTPRequester(scriptHTTPRequester(func(_ context.Context, nodes []domain.NodeIR, opts domain.NodeHTTPOptions) ([]domain.NodeHTTPResult, error) {
		calls++
		require.Equal(t, "GET", opts.Method)
		require.Equal(t, "https://www.cloudflare.com/cdn-cgi/trace", opts.URL)
		results := make([]domain.NodeHTTPResult, len(nodes))
		for i, node := range nodes {
			body := "ip=8.8.8.8\nloc=JP\n"
			if node.Server == "invalid.example.com" {
				body = "ip=invalid\nloc=JP\n"
			}
			if node.Server == "unknown.example.com" {
				body = "ip=8.8.8.8\nloc=XX\n"
			}
			results[i] = domain.NodeHTTPResult{Index: i, StatusCode: 200, Body: body}
		}
		return results, nil
	})))
	proc, err := registry.BuildNode(domain.ProcessorSpec{Type: "script", Stage: domain.StageNodes, Params: params(t, map[string]any{"timeout_ms": 300000, "content": exitInfoSource(t)})})
	require.NoError(t, err)
	input := []domain.NodeIR{{Name: "a", Type: domain.NodeTypeHTTP, Server: "a.example.com", Port: 80}, {Name: "bad", Type: domain.NodeTypeHTTP, Server: "invalid.example.com", Port: 80}, {Name: "unknown", Type: domain.NodeTypeHTTP, Server: "unknown.example.com", Port: 80}, {Name: "b", Type: domain.NodeTypeHTTP, Server: "b.example.com", Port: 80}}
	out, err := proc.ApplyNodes(context.Background(), domain.NodeProcessInput{Nodes: input})
	require.NoError(t, err)
	require.Equal(t, 1, calls)
	require.Len(t, out.Nodes, 2)
	require.Len(t, out.Warnings, 2)
	for _, node := range out.Nodes {
		require.Regexp(t, `^JP-[0-9a-f]{6}$`, node.Name)
		require.Equal(t, "8.8.8.8", node.Meta["exit.ip"])
		require.Equal(t, "JP", node.Meta["exit.country"])
	}
	again, err := proc.ApplyNodes(context.Background(), domain.NodeProcessInput{Nodes: []domain.NodeIR{out.Nodes[1], out.Nodes[0]}})
	require.NoError(t, err)
	require.Equal(t, out.Nodes[0].Name, again.Nodes[1].Name)
	require.Equal(t, out.Nodes[1].Name, again.Nodes[0].Name)
}

func TestExampleExitInfoHashCollisionUsesStableSuffix(t *testing.T) {
	vm := goja.New()
	_, err := vm.RunString(exitInfoSource(t) + `
 var calls=0;
 var api={http:{request:function(nodes, options){calls++;return nodes.map(function(_,i){return {index:i,status_code:200,body:"ip=8.8.8.8\\nloc=JP\\n".replace(/\\n/g,"\n")};});}},hash:{sha256:function(){return "abcdef000000";}},warn:function(){}};
 var a={name:"same",type:"http",server:"a.example.com",port:80};
 var b={name:"same",type:"http",server:"b.example.com",port:80};
 var result=main({nodes:[b,a]},api);
 JSON.stringify([result.nodes[0].name,result.nodes[1].name,calls]);`)
	require.NoError(t, err)
	value, err := vm.RunString(`JSON.stringify([result.nodes[0].name,result.nodes[1].name,calls])`)
	require.NoError(t, err)
	require.Equal(t, `["JP-abcdef-2","JP-abcdef",1]`, value.String())
}

func TestExampleExitInfoValidatesIPv4IPv6AndCountry(t *testing.T) {
	vm := goja.New()
	_, err := vm.RunString(exitInfoSource(t))
	require.NoError(t, err)
	validIP, ok := goja.AssertFunction(vm.Get("validIP"))
	require.True(t, ok)
	for _, ip := range []string{"8.8.8.8", "2001:db8::1", "::", "::ffff:8.8.8.8"} {
		v, e := validIP(goja.Undefined(), vm.ToValue(ip))
		require.NoError(t, e)
		require.True(t, v.ToBoolean(), ip)
	}
	for _, ip := range []string{"999.0.0.1", "01.2.3.4", "2001:db8:::1", "1:2:3:4:5:6:7:8:9", "not-ip", "::ffff:999.0.0.1"} {
		v, e := validIP(goja.Undefined(), vm.ToValue(ip))
		require.NoError(t, e)
		require.False(t, v.ToBoolean(), ip)
	}
}

func TestExampleExitInfoRejectsNonPublicAndAmbiguousResponses(t *testing.T) {
	vm := goja.New()
	_, err := vm.RunString(exitInfoSource(t))
	require.NoError(t, err)
	check, ok := goja.AssertFunction(vm.Get("publicIP"))
	require.True(t, ok)
	for _, ip := range []string{"8.8.8.8", "2001:4860:4860::8888", "::ffff:8.8.8.8", "::ffff:0808:0808"} {
		v, e := check(goja.Undefined(), vm.ToValue(ip))
		require.NoError(t, e)
		require.True(t, v.ToBoolean(), ip)
	}
	for _, ip := range []string{"0.0.0.0", "10.0.0.1", "127.0.0.1", "169.254.1.1", "172.16.0.1", "192.168.0.1", "100.64.0.1", "224.0.0.1", "255.255.255.255", "192.0.2.1", "198.51.100.1", "203.0.113.1", "::", "::1", "fe80::1", "fc00::1", "ff02::1", "2001:db8::1", "3fff::1", "::ffff:127.0.0.1", "::ffff:7f00:1"} {
		v, e := check(goja.Undefined(), vm.ToValue(ip))
		require.NoError(t, e)
		require.False(t, v.ToBoolean(), ip)
	}
	value, err := vm.RunString(`var warnings=[];var result=main({nodes:[{name:"n",type:"http",server:"example.com",port:80}]},{http:{request:function(){return [{index:0,status_code:200,body:"ip=8.8.8.8\nloc=XX\nloc=US\n"}];}},warn:function(w){warnings.push(w);}});result.nodes.length===0 && warnings.length===1;`)
	require.NoError(t, err)
	require.True(t, value.ToBoolean())
}
