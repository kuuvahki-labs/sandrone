package script_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/processor"
	"github.com/kuuvahki-labs/sandrone/internal/processor/script"
)

type scriptHTTPRequester func(context.Context, []domain.NodeIR, domain.NodeHTTPOptions) ([]domain.NodeHTTPResult, error)

func (f scriptHTTPRequester) RequestNodeHTTP(ctx context.Context, nodes []domain.NodeIR, opts domain.NodeHTTPOptions) ([]domain.NodeHTTPResult, error) {
	return f(ctx, nodes, opts)
}

func TestScriptHTTPExactInputSubsetAndSerialization(t *testing.T) {
	calls := 0
	registry := processor.NewRegistry()
	registerScript(registry, script.WithHTTPRequester(scriptHTTPRequester(func(_ context.Context, nodes []domain.NodeIR, opts domain.NodeHTTPOptions) ([]domain.NodeHTTPResult, error) {
		calls++
		require.Len(t, nodes, 1)
		require.Equal(t, "second", nodes[0].Name)
		require.Equal(t, "https://example.com/", opts.URL)
		require.Equal(t, "mihomo", opts.Core)
		require.Equal(t, "test", opts.Headers["X-Test"])
		return []domain.NodeHTTPResult{{Index: 0, StatusCode: 200, Body: "ip=192.0.2.1", Headers: map[string][]string{"X-Test": {"ok"}}}}, nil
	})))
	proc, err := registry.BuildNode(domain.ProcessorSpec{Type: "script", Stage: domain.StageNodes, Params: params(t, map[string]any{"content": `
 function main(input,api) {
   var result=api.http.request([input.nodes[1]],{url:"https://example.com/",core:"mihomo",headers:{"X-Test":"test"}});
   if(result[0].index!==0 || result[0].status_code!==200 || result[0].body!=="ip=192.0.2.1" || result[0].headers["X-Test"][0]!=="ok") throw new Error("result shape");
   input.nodes=[input.nodes[1]]; return input;
 }`})})
	require.NoError(t, err)
	out, err := proc.ApplyNodes(context.Background(), domain.NodeProcessInput{Nodes: []domain.NodeIR{{Name: "first", Type: domain.NodeTypeHTTP, Server: "one.example.com", Port: 80}, {Name: "second", Type: domain.NodeTypeHTTP, Server: "two.example.com", Port: 80}}})
	require.NoError(t, err)
	require.Len(t, out.Nodes, 1)
	require.Equal(t, 1, calls)
}

func TestScriptHTTPRejectsModifiedInventedRepeatedAndInvalidInputs(t *testing.T) {
	for name, body := range map[string]string{
		"modified":       `input.nodes[0].server="other.example.com";api.http.request(input.nodes,{url:"https://example.com/"});`,
		"invented":       `api.http.request([{name:"new",type:"http",server:"example.com",port:80}],{url:"https://example.com/"});`,
		"duplicates":     `api.http.request([input.nodes[0],input.nodes[0]],{url:"https://example.com/"});`,
		"missing":        `api.http.request();`,
		"null":           `api.http.request(null,{url:"https://example.com/"});`,
		"unknown option": `api.http.request(input.nodes,{url:"https://example.com/",typo:1});`,
	} {
		t.Run(name, func(t *testing.T) {
			registry := processor.NewRegistry()
			registerScript(registry, script.WithHTTPRequester(scriptHTTPRequester(func(context.Context, []domain.NodeIR, domain.NodeHTTPOptions) ([]domain.NodeHTTPResult, error) {
				t.Error("invalid input reached HTTP requester")
				return nil, nil
			})))
			proc, err := registry.BuildNode(domain.ProcessorSpec{Type: "script", Stage: domain.StageNodes, Params: params(t, map[string]any{"content": "function main(input,api){" + body + "return input;}"})})
			require.NoError(t, err)
			_, err = proc.ApplyNodes(context.Background(), domain.NodeProcessInput{Nodes: []domain.NodeIR{{Name: "original", Type: domain.NodeTypeHTTP, Server: "example.com", Port: 80}}})
			require.Error(t, err)
		})
	}
}

func TestScriptHTTPRequestErrorAndTimeout(t *testing.T) {
	for _, wait := range []bool{false, true} {
		t.Run(map[bool]string{false: "error", true: "timeout"}[wait], func(t *testing.T) {
			registry := processor.NewRegistry()
			registerScript(registry, script.WithHTTPRequester(scriptHTTPRequester(func(ctx context.Context, _ []domain.NodeIR, _ domain.NodeHTTPOptions) ([]domain.NodeHTTPResult, error) {
				if wait {
					<-ctx.Done()
					return nil, ctx.Err()
				}
				return nil, errors.New("core start failed")
			})))
			proc, err := registry.BuildNode(domain.ProcessorSpec{Type: "script", Stage: domain.StageNodes, Params: params(t, map[string]any{"timeout_ms": 20, "content": `function main(input,api){api.http.request(input.nodes,{url:"https://example.com/"});return input;}`})})
			require.NoError(t, err)
			_, err = proc.ApplyNodes(context.Background(), domain.NodeProcessInput{Nodes: []domain.NodeIR{{Name: "node", Type: domain.NodeTypeHTTP, Server: "example.com", Port: 80}}})
			require.Error(t, err)
			if wait {
				require.True(t, domain.IsCode(err, domain.CodeScriptTimeout))
			} else {
				require.Contains(t, err.Error(), "core start failed")
			}
		})
	}
}

func TestScriptHTTPNotExposedAtFileStage(t *testing.T) {
	registry := processor.NewRegistry()
	registerScript(registry, script.WithHTTPRequester(scriptHTTPRequester(func(context.Context, []domain.NodeIR, domain.NodeHTTPOptions) ([]domain.NodeHTTPResult, error) {
		t.Fatal("must not run")
		return nil, nil
	})))
	proc, err := registry.BuildFile(domain.ProcessorSpec{Type: "script", Stage: domain.StageFile, Params: params(t, map[string]any{"content": `function main(input,api){if(typeof api.http!=="undefined")throw new Error("HTTP exposed to file stage");return input;}`})})
	require.NoError(t, err)
	_, err = proc.ApplyFile(context.Background(), domain.FileProcessInput{})
	require.NoError(t, err)
}
