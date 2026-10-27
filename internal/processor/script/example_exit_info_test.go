package script_test

import (
	"context"
	"fmt"
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

func TestExampleExitInfoOnlyAnnotatesOneRequestPerNode(t *testing.T) {
	input := make([]domain.NodeIR, 6)
	for i := range input {
		input[i] = domain.NodeIR{
			Name: "unchanged", Type: domain.NodeTypeHTTP, Server: fmt.Sprintf("n%d.example.com", i), Port: 8080,
			Username: "user", Password: "fixture", Tags: []string{"existing"},
			TLS:  &domain.TLSOptions{Enabled: true, ServerName: "proxy.example.com"},
			Meta: map[string]string{"owner": "keep", "exit.note": "keep", "exit.valid": "true", "exit.ip": "9.9.9.9", "exit.country": "GB", "exit.duration_ms": "999"},
		}
	}
	calls := 0
	requested := make(map[string]int)
	probeRunner := &scriptProbeRunner{}
	registry := processor.NewRegistry()
	registerScript(registry, script.WithProbeRunner(probeRunner), script.WithHTTPRequester(scriptHTTPRequester(func(_ context.Context, nodes []domain.NodeIR, opts domain.NodeHTTPOptions) ([]domain.NodeHTTPResult, error) {
		calls++
		require.Equal(t, input, nodes, "request must receive the original unmodified nodes and metadata")
		require.Equal(t, "GET", opts.Method)
		require.Equal(t, "https://www.cloudflare.com/cdn-cgi/trace", opts.URL)
		results := make([]domain.NodeHTTPResult, len(nodes))
		for i, node := range nodes {
			requested[node.Server]++
			result := domain.NodeHTTPResult{Index: i, StatusCode: 200, Body: "ip=8.8.8.8\nloc=JP\n", DurationMS: 30}
			switch i {
			case 1:
				result.Error = "request failed"
			case 2:
				result.StatusCode = 503
			case 3:
				result.Body = "ip=invalid\nloc=JP\n"
			case 4:
				result.Body = "ip=8.8.8.8\nloc=XX\n"
			case 5:
				result.Body, result.DurationMS = "ip=1.1.1.1\nloc=US\n", 1
			}
			results[len(nodes)-1-i] = result
		}
		return results, nil
	})))
	proc, err := registry.BuildNode(domain.ProcessorSpec{Type: "script", Stage: domain.StageNodes, Params: params(t, map[string]any{
		"content": exitInfoSource(t),
	})})
	require.NoError(t, err)
	out, err := proc.ApplyNodes(t.Context(), domain.NodeProcessInput{Nodes: input})
	require.NoError(t, err)
	require.Equal(t, 1, calls)
	require.Empty(t, probeRunner.requests)
	require.Len(t, requested, len(input))
	for _, count := range requested {
		require.Equal(t, 1, count)
	}
	require.Len(t, out.Nodes, len(input), "exit-info must not filter or limit nodes")
	require.Len(t, out.Warnings, 4)
	for i, node := range out.Nodes {
		original, observed := input[i], node
		original.Meta, observed.Meta = nil, nil
		require.Equal(t, original, observed, "node order, names and connection fields must not change")
		require.Equal(t, "keep", node.Meta["owner"])
		require.Equal(t, "keep", node.Meta["exit.note"])
		if i == 0 || i == 5 {
			require.Equal(t, "true", node.Meta["exit.valid"])
		} else {
			require.Equal(t, "false", node.Meta["exit.valid"])
			require.NotContains(t, node.Meta, "exit.ip")
			require.NotContains(t, node.Meta, "exit.country")
			require.NotContains(t, node.Meta, "exit.duration_ms")
		}
		require.Equal(t, "9.9.9.9", input[i].Meta["exit.ip"], "processing must not mutate caller input")
	}
	require.Equal(t, "8.8.8.8", out.Nodes[0].Meta["exit.ip"])
	require.Equal(t, "JP", out.Nodes[0].Meta["exit.country"])
	require.Equal(t, "30", out.Nodes[0].Meta["exit.duration_ms"])
	require.Equal(t, "1.1.1.1", out.Nodes[5].Meta["exit.ip"])
	require.Equal(t, "US", out.Nodes[5].Meta["exit.country"])
	require.Equal(t, "1", out.Nodes[5].Meta["exit.duration_ms"])
	for i, warning := range out.Warnings {
		require.Equal(t, "exit_info_invalid", warning.Code)
		require.Equal(t, i+1, *warning.NodeIndex)
	}
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
	value, err := vm.RunString(`var warnings=[];var result=main({nodes:[{name:"n",type:"http",server:"example.com",port:80}]},{http:{request:function(){return [{index:0,status_code:200,duration_ms:10,body:"ip=8.8.8.8\nloc=XX\nloc=US\n"}];}},warn:function(w){warnings.push(w);}});result.nodes.length===1 && result.nodes[0].name==="n" && result.nodes[0].meta["exit.valid"]==="false" && warnings.length===1;`)
	require.NoError(t, err)
	require.True(t, value.ToBoolean())
}

func TestExampleExitInfoInvalidDurationsRetainNodesAndClearOldValues(t *testing.T) {
	vm := goja.New()
	value, err := vm.RunString(exitInfoSource(t) + `
 var durations=[undefined,null,"0",NaN,Infinity,-Infinity,-1,true,{},[],0,0.5];
 var warnings=[],calls=0;
 var nodes=durations.map(function(_,i){return {name:"n"+i,type:"http",server:"n"+i,port:80,meta:{"exit.valid":"true","exit.ip":"9.9.9.9","exit.country":"GB","exit.duration_ms":"99"}};});
 var result=main({nodes:nodes},{http:{request:function(){calls++;return nodes.map(function(_,i){return {index:i,status_code:200,duration_ms:durations[i],body:"ip=8.8.8.8\nloc=JP\n"};});}},
   warn:function(w){warnings.push(w);}});
 result.nodes.length===durations.length && result.nodes.every(function(n,i){
   return n.name==="n"+i && n.server==="n"+i && n.meta["exit.valid"]===(i<10 ? "false":"true") &&
     (i<10 ? !Object.prototype.hasOwnProperty.call(n.meta,"exit.ip") && !Object.prototype.hasOwnProperty.call(n.meta,"exit.country") && !Object.prototype.hasOwnProperty.call(n.meta,"exit.duration_ms") : n.meta["exit.duration_ms"]===String(durations[i]));
 });`)
	require.NoError(t, err)
	require.True(t, value.ToBoolean())
	value, err = vm.RunString(`calls===1 && warnings.length===10 && warnings.every(function(w,i){return w.code==="exit_info_invalid" && w.node_index===i;})`)
	require.NoError(t, err)
	require.True(t, value.ToBoolean())
}

func TestExampleExitInfoMissingResultCannotReusePreviousObservation(t *testing.T) {
	vm := goja.New()
	value, err := vm.RunString(exitInfoSource(t) + `
 var warnings=[],calls=0;
 var node={name:"keep",type:"http",server:"proxy.example.com",port:80,meta:{"exit.valid":"true","exit.ip":"9.9.9.9","exit.country":"GB","exit.duration_ms":"99","owner":"keep"}};
 var input={nodes:[node]},result=main(input,{http:{request:function(){calls++;return [];}},warn:function(w){warnings.push(w);}});
 result===input && result.nodes.length===1 && result.nodes[0]===node && node.name==="keep" &&
 JSON.stringify(node.meta)===JSON.stringify({"exit.valid":"false","owner":"keep"}) && calls===1 && warnings.length===1 && warnings[0].node_index===0;`)
	require.NoError(t, err)
	require.True(t, value.ToBoolean())
}
