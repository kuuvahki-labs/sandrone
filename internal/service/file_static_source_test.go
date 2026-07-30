package service_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/service"
)

func TestServiceFileInlineSourceOutputsContent(t *testing.T) {
	svc := service.New()
	spec := domain.FileSpec{
		Name: "hello.txt",
		Kind: domain.FileKindStatic,
		Source: domain.FileSource{
			Type:    "inline",
			Content: "hello",
		},
	}

	result, err := svc.GetFile(context.Background(), domain.FileRequest{Spec: &spec})

	require.NoError(t, err)
	require.Equal(t, "hello", string(result.File.Content))
	require.Equal(t, "hello", string(result.Content))
	require.Equal(t, "file", result.Report.Kind)
}

func TestServiceRejectsLocalFileSource(t *testing.T) {
	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	spec := domain.FileSpec{
		Name:   "manual.txt",
		Kind:   domain.FileKindStatic,
		Source: domain.FileSource{Type: "local"},
	}

	err := svc.PutFile(context.Background(), spec)
	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeInvalidArgument), "got %v", err)
	require.Contains(t, err.Error(), "inline or remote")

	_, err = svc.GetFile(context.Background(), domain.FileRequest{Spec: &spec})
	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeInvalidArgument), "got %v", err)
	require.Contains(t, err.Error(), "inline or remote")
}

func TestServiceRejectsEmptyStaticFileSource(t *testing.T) {
	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	spec := domain.FileSpec{Name: "manual.txt", Kind: domain.FileKindStatic}

	err := svc.PutFile(context.Background(), spec)
	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeInvalidArgument), "got %v", err)
	require.Contains(t, err.Error(), "requires an inline or remote source")
}

func TestServiceRejectsTypedBuiltinSourceWithPayload(t *testing.T) {
	for _, test := range []struct {
		name   string
		source domain.FileSource
	}{
		{
			name:   "content",
			source: domain.FileSource{Content: "ignored"},
		},
		{
			name:   "remote",
			source: domain.FileSource{Remote: &domain.RemoteInput{URL: "https://example.com/ignored"}},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			svc := service.New(service.WithFS(afero.NewMemMapFs()))
			err := svc.PutFile(context.Background(), domain.FileSpec{
				Name:   "default.yaml",
				Kind:   domain.FileKindMihomo,
				Source: test.source,
				Config: &domain.FileConfig{},
			})

			require.Error(t, err)
			require.True(t, domain.IsCode(err, domain.CodeInvalidArgument), "got %v", err)
			require.Contains(t, err.Error(), "empty file source must not include content or remote")
		})
	}
}

func TestServiceFileRemoteSourceFetchesEachRender(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = fmt.Fprintf(w, "remote-%d", calls)
	}))
	defer server.Close()

	svc := service.New()
	spec := domain.FileSpec{
		Name: "remote.txt",
		Kind: domain.FileKindStatic,
		Source: domain.FileSource{
			Type:   "remote",
			Remote: &domain.RemoteInput{URL: server.URL},
		},
	}

	first, err := svc.GetFile(context.Background(), domain.FileRequest{Spec: &spec})
	require.NoError(t, err)
	second, err := svc.GetFile(context.Background(), domain.FileRequest{Spec: &spec})
	require.NoError(t, err)

	require.Equal(t, "remote-1", string(first.File.Content))
	require.Equal(t, "remote-2", string(second.File.Content))
	require.Equal(t, 2, calls)
	require.Len(t, second.Report.SourceRefs, 1)
	require.Equal(t, server.URL, second.Report.SourceRefs[0].URL)
}

func TestServiceFileRemoteSourceUsesCacheTTL(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = fmt.Fprintf(w, "remote-%d", calls)
	}))
	defer server.Close()

	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	spec := domain.FileSpec{
		Name: "remote.txt",
		Kind: domain.FileKindStatic,
		Source: domain.FileSource{
			Type:   "remote",
			Remote: &domain.RemoteInput{URL: server.URL, CacheTTLSeconds: 60},
		},
	}

	first, err := svc.GetFile(context.Background(), domain.FileRequest{Spec: &spec})
	require.NoError(t, err)
	second, err := svc.GetFile(context.Background(), domain.FileRequest{Spec: &spec})
	require.NoError(t, err)

	require.Equal(t, "remote-1", string(first.File.Content))
	require.Equal(t, "remote-1", string(second.File.Content))
	require.Equal(t, 1, calls)
	require.Contains(t, second.Report.SourceRefs[0].Note, "cache_hit=true")
}

func TestServiceRemoteNodeInputUsesCacheTTL(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = fmt.Fprintf(w, "ss://aes-128-gcm:secret@example.com:8388#remote-%d", calls)
	}))
	defer server.Close()

	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	require.NoError(t, svc.PutSubscription(context.Background(), domain.Subscription{
		Name: "groups/live",
		Type: domain.SubscriptionTypeCollection,
		Inputs: []domain.NodeInput{{
			Name:            "remote",
			Type:            "remote",
			Format:          "uri-list",
			URL:             server.URL,
			CacheTTLSeconds: 60,
		}},
	}))

	first, err := svc.PreviewSubscription(context.Background(), "groups/live")
	require.NoError(t, err)
	second, err := svc.PreviewSubscription(context.Background(), "groups/live")
	require.NoError(t, err)

	require.Equal(t, 1, calls)
	require.Equal(t, "remote-1", first.Nodes[0].Before.Name)
	require.Equal(t, "remote-1", second.Nodes[0].Before.Name)
}

func TestServiceScriptProcessorLoadsScriptFromFileResource(t *testing.T) {
	ctx := context.Background()
	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	require.NoError(t, svc.PutFile(ctx, domain.FileSpec{
		Name: "rename.js",
		Kind: domain.FileKindStatic,
		Source: domain.FileSource{
			Type: "inline",
			Content: `function main(input) {
  input.nodes.forEach(function(node) { node.name = "__SOURCE_PREFIX____SOURCE_OUTER__" + input.args.prefix + node.name; });
  return input;
}`,
		},
		Processors: []domain.ProcessorSpec{{
			Type:  "script",
			Stage: domain.StageFile,
			Params: params(t, map[string]any{
				"source": inlineScriptSource(`function main(input) {
  input.file.content = input.file.content.replace("__SOURCE_PREFIX__", input.args.prefix);
  input.file.content = input.file.content.replace("__SOURCE_OUTER__", input.args.outer || "isolated-");
  return input;
}`),
			}),
		}},
	}))
	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
		Name:    "provider",
		Type:    domain.SubscriptionTypeLocal,
		Format:  "uri-list",
		Content: "ss://aes-128-gcm:secret@example.com:8388#node-a",
		Processors: []domain.ProcessorSpec{{
			Type:  "script",
			Stage: domain.StageNodes,
			Params: params(t, map[string]any{
				"source": map[string]any{
					"type": "file",
					"name": "rename.js",
					"args": map[string]string{"prefix": "rendered-"},
				},
				"args": map[string]any{"prefix": "params-"},
			}),
		}},
	}))

	preview, err := svc.PreviewSubscription(ctx, "provider", map[string]string{"outer": "leaked-"})

	require.NoError(t, err)
	require.Len(t, preview.Nodes, 1)
	require.Equal(t, "node-a", preview.Nodes[0].Before.Name)
	require.Equal(t, "rendered-isolated-params-node-a", preview.Nodes[0].After.Name)
}

func TestServiceScriptProcessorLoadsStructuredFileSourceFromRemoteFileResource(t *testing.T) {
	ctx := context.Background()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`function main(input) {
  input.nodes.forEach(function(node) { node.name = "remote-file-" + node.name; });
  return input;
}`))
	}))
	defer server.Close()

	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	require.NoError(t, svc.PutFile(ctx, domain.FileSpec{
		Name: "rename.js",
		Kind: domain.FileKindStatic,
		Source: domain.FileSource{
			Type:   "remote",
			Remote: &domain.RemoteInput{URL: server.URL},
		},
		Processors: []domain.ProcessorSpec{{
			Type:  "script",
			Stage: domain.StageFile,
			Params: params(t, map[string]any{
				"source": inlineScriptSource(`function main(input) {
  input.file.content = input.file.content.replace("remote-file-", "processed-remote-file-");
  return input;
}`),
			}),
		}},
	}))
	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
		Name:    "provider",
		Type:    domain.SubscriptionTypeLocal,
		Format:  "uri-list",
		Content: "ss://aes-128-gcm:secret@example.com:8388#node-a",
		Processors: []domain.ProcessorSpec{{
			Type:  "script",
			Stage: domain.StageNodes,
			Params: params(t, map[string]any{
				"source": map[string]any{"type": "file", "name": "rename.js"},
			}),
		}},
	}))

	preview, err := svc.PreviewSubscription(ctx, "provider")

	require.NoError(t, err)
	require.Len(t, preview.Nodes, 1)
	require.Equal(t, "processed-remote-file-node-a", preview.Nodes[0].After.Name)
}

func TestServiceScriptProcessorLoadsStructuredRemoteSource(t *testing.T) {
	ctx := context.Background()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`function main(input) {
  input.nodes.forEach(function(node) { node.name = "direct-remote-" + node.name; });
  return input;
}`))
	}))
	defer server.Close()

	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
		Name:    "provider",
		Type:    domain.SubscriptionTypeLocal,
		Format:  "uri-list",
		Content: "ss://aes-128-gcm:secret@example.com:8388#node-a",
		Processors: []domain.ProcessorSpec{{
			Type:  "script",
			Stage: domain.StageNodes,
			Params: params(t, map[string]any{
				"source": map[string]any{
					"type":   "remote",
					"remote": map[string]any{"url": server.URL},
				},
			}),
		}},
	}))

	preview, err := svc.PreviewSubscription(ctx, "provider")

	require.NoError(t, err)
	require.Len(t, preview.Nodes, 1)
	require.Equal(t, "direct-remote-node-a", preview.Nodes[0].After.Name)
}

func TestServiceScriptProcessorRemoteSourceUsesRuntimeDefaults(t *testing.T) {
	ctx := context.Background()
	userAgents := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userAgents = append(userAgents, r.UserAgent())
		_, _ = w.Write([]byte(`function main(input) {
  input.nodes.forEach(function(node) { node.name = "direct-remote-" + node.name; });
  return input;
}`))
	}))
	defer server.Close()

	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	putProjectSettings(t, svc, ctx, func(update *domain.SettingsUpdate) {
		update.RemoteDefaults = domain.RemoteDefaults{
			UserAgent: "Sandrone Global",
			TimeoutMS: 8000,
		}
	})
	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
		Name:    "provider",
		Type:    domain.SubscriptionTypeLocal,
		Format:  "uri-list",
		Content: "ss://aes-128-gcm:secret@example.com:8388#node-a",
		Processors: []domain.ProcessorSpec{{
			Type:  "script",
			Stage: domain.StageNodes,
			Params: params(t, map[string]any{
				"source": map[string]any{
					"type":   "remote",
					"remote": map[string]any{"url": server.URL},
				},
			}),
		}},
	}))

	preview, err := svc.PreviewSubscription(ctx, "provider")

	require.NoError(t, err)
	require.Len(t, preview.Nodes, 1)
	require.Equal(t, "direct-remote-node-a", preview.Nodes[0].After.Name)
	require.Equal(t, []string{"Sandrone Global"}, userAgents)
}

func TestServiceStoredSubscriptionPreservesExplicitLegacyLookingUserAgent(t *testing.T) {
	ctx := context.Background()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "sandrone/0", r.UserAgent())
		_, _ = w.Write([]byte("ss://aes-128-gcm:secret@example.com:8388#remote-node"))
	}))
	defer server.Close()

	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
		Name:   "provider",
		Type:   domain.SubscriptionTypeRemote,
		Format: "uri-list",
		Remote: &domain.RemoteInput{
			URL:       server.URL,
			UserAgent: "sandrone/0",
		},
	}))

	preview, err := svc.PreviewSubscription(ctx, "provider")

	require.NoError(t, err)
	require.Len(t, preview.Nodes, 1)
}

func TestServiceScriptProcessorRejectsRemoteSourceSHA256Mismatch(t *testing.T) {
	ctx := context.Background()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`function main(input) { return input; }`))
	}))
	defer server.Close()

	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
		Name:    "provider",
		Type:    domain.SubscriptionTypeLocal,
		Format:  "uri-list",
		Content: "ss://aes-128-gcm:secret@example.com:8388#node-a",
		Processors: []domain.ProcessorSpec{{
			Type:  "script",
			Stage: domain.StageNodes,
			Params: params(t, map[string]any{
				"source": map[string]any{
					"type":   "remote",
					"remote": map[string]any{"url": server.URL},
					"sha256": "0000000000000000000000000000000000000000000000000000000000000000",
				},
			}),
		}},
	}))

	_, err := svc.PreviewSubscription(ctx, "provider")

	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeProcessorConfigInvalid), "got %v", err)
}

func TestServiceScriptProcessorRejectsUnsafeFileResourcePath(t *testing.T) {
	ctx := context.Background()
	svc := service.New(service.WithFS(afero.NewMemMapFs()))

	for _, scriptPath := range []string{"/files/rename.js", "../rename.js", `files\rename.js`} {
		t.Run(scriptPath, func(t *testing.T) {
			require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
				Name:    "provider-" + strings.NewReplacer("/", "-", "\\", "-").Replace(scriptPath),
				Type:    domain.SubscriptionTypeLocal,
				Format:  "uri-list",
				Content: "ss://aes-128-gcm:secret@example.com:8388#node-a",
				Processors: []domain.ProcessorSpec{{
					Type:  "script",
					Stage: domain.StageNodes,
					Params: params(t, map[string]any{
						"source": fileScriptSource(scriptPath),
					}),
				}},
			}))

			_, err := svc.PreviewSubscription(ctx, "provider-"+strings.NewReplacer("/", "-", "\\", "-").Replace(scriptPath))
			require.Error(t, err)
			require.True(t, domain.IsCode(err, domain.CodeProcessorConfigInvalid), "got %v", err)
		})
	}
}

func TestServiceFileScriptReceivesRequestArgs(t *testing.T) {
	svc := service.New()
	spec := domain.FileSpec{
		Name: "args.txt",
		Kind: domain.FileKindStatic,
		Source: domain.FileSource{
			Type:    "inline",
			Content: "hello",
		},
		Processors: []domain.ProcessorSpec{{
			Type:  "script",
			Stage: domain.StageFile,
			Params: params(t, map[string]any{
				"source": inlineScriptSource(`function main(input) {
  input.file.content = input.file.content + ":" + input.args.name;
  return input;
}`),
			}),
		}},
	}

	result, err := svc.GetFile(context.Background(), domain.FileRequest{
		Spec:    &spec,
		Request: domain.RequestInfo{Args: map[string]string{"name": "world"}},
	})

	require.NoError(t, err)
	require.Equal(t, "hello:world", string(result.File.Content))
}

func TestServiceFileScriptReadsAnotherProcessedFile(t *testing.T) {
	ctx := context.Background()
	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	require.NoError(t, svc.PutFile(ctx, domain.FileSpec{
		Name: "child.txt",
		Kind: domain.FileKindStatic,
		Source: domain.FileSource{
			Type:    "inline",
			Content: "child",
		},
		Processors: []domain.ProcessorSpec{{
			Type:  "script",
			Stage: domain.StageFile,
			Params: params(t, map[string]any{
				"source": inlineScriptSource(`function main(input) {
  input.file.content = input.file.content + ((input.args && input.args.suffix) || "");
  return input;
}`),
			}),
		}},
	}))

	parent := domain.FileSpec{
		Name: "parent.txt",
		Kind: domain.FileKindStatic,
		Source: domain.FileSource{
			Type:    "inline",
			Content: "parent:",
		},
		Processors: []domain.ProcessorSpec{{
			Type:  "script",
			Stage: domain.StageFile,
			Params: params(t, map[string]any{
				"source": inlineScriptSource(`function main(input, api) {
  var omitted = api.file.content("child.txt");
  var nullable = api.file.content("child.txt", null);
  var ignored = api.file.content("child.txt", {args: {suffix: "!"}}, "ignored");
  input.file.content = input.file.content + omitted + ":" + nullable + ":" + ignored;
  return input;
}`),
			}),
		}},
	}

	result, err := svc.GetFile(ctx, domain.FileRequest{
		Spec:    &parent,
		Request: domain.RequestInfo{Args: map[string]string{"suffix": "-outer"}},
	})

	require.NoError(t, err)
	require.Equal(t, "parent:child:child:child!", string(result.File.Content))
	require.Contains(t, result.Report.Dependencies, domain.ResourceRef{Kind: "file", Name: "child.txt"})
}

func TestServiceFileScriptDependencyCycleFails(t *testing.T) {
	ctx := context.Background()
	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	require.NoError(t, svc.PutFile(ctx, domain.FileSpec{
		Name:   "a.txt",
		Kind:   domain.FileKindStatic,
		Source: domain.FileSource{Type: "inline", Content: "a"},
		Processors: []domain.ProcessorSpec{{
			Type:  "script",
			Stage: domain.StageFile,
			Params: params(t, map[string]any{
				"source": inlineScriptSource(`function main(input, api) {
  input.file.content = input.file.content + api.file.content("b.txt");
  return input;
}`),
			}),
		}},
	}))
	require.NoError(t, svc.PutFile(ctx, domain.FileSpec{
		Name:   "b.txt",
		Kind:   domain.FileKindStatic,
		Source: domain.FileSource{Type: "inline", Content: "b"},
		Processors: []domain.ProcessorSpec{{
			Type:  "script",
			Stage: domain.StageFile,
			Params: params(t, map[string]any{
				"source": inlineScriptSource(`function main(input, api) {
  input.file.content = input.file.content + api.file.content("a.txt");
  return input;
}`),
			}),
		}},
	}))

	_, err := svc.GetFile(ctx, domain.FileRequest{Name: "a.txt"})

	require.Error(t, err)
	require.True(t, domain.IsCode(err, domain.CodeFileDependencyCycle), "got %v", err)
}

func TestServiceScriptFileSourceDependencyCyclesFail(t *testing.T) {
	tests := map[string][]domain.FileSpec{
		"direct": {{
			Name:   "a.js",
			Kind:   domain.FileKindStatic,
			Source: domain.FileSource{Type: "inline", Content: "function main(input) { return input; }"},
			Processors: []domain.ProcessorSpec{{
				Type:  "script",
				Stage: domain.StageFile,
				Params: params(t, map[string]any{
					"source": fileScriptSource("a.js"),
				}),
			}},
		}},
		"indirect": {
			{
				Name:   "a.js",
				Kind:   domain.FileKindStatic,
				Source: domain.FileSource{Type: "inline", Content: "function main(input) { return input; }"},
				Processors: []domain.ProcessorSpec{{
					Type:  "script",
					Stage: domain.StageFile,
					Params: params(t, map[string]any{
						"source": fileScriptSource("b.js"),
					}),
				}},
			},
			{
				Name:   "b.js",
				Kind:   domain.FileKindStatic,
				Source: domain.FileSource{Type: "inline", Content: "function main(input) { return input; }"},
				Processors: []domain.ProcessorSpec{{
					Type:  "script",
					Stage: domain.StageFile,
					Params: params(t, map[string]any{
						"source": fileScriptSource("a.js"),
					}),
				}},
			},
		},
	}

	for name, specs := range tests {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			svc := service.New(service.WithFS(afero.NewMemMapFs()))
			for _, spec := range specs {
				require.NoError(t, svc.PutFile(ctx, spec))
			}

			_, err := svc.GetFile(ctx, domain.FileRequest{Name: "a.js"})

			require.Error(t, err)
			require.True(t, domain.IsCode(err, domain.CodeFileDependencyCycle), "got %v", err)
		})
	}
}

func TestServiceFileScriptProducesSubscriptionsAsNodesAndContent(t *testing.T) {
	ctx := context.Background()
	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
		Name:    "sub",
		Type:    domain.SubscriptionTypeLocal,
		Format:  "uri-list",
		Content: "ss://aes-128-gcm:secret@example.com:8388#node-a",
		Processors: []domain.ProcessorSpec{{
			Type:  "script",
			Stage: domain.StageNodes,
			Params: params(t, map[string]any{
				"source": inlineScriptSource(`function main(input) {
  var prefix = (input.args && input.args.prefix) || "";
  input.nodes.forEach(function(node) { node.name = prefix + node.name; });
  return input;
}`),
			}),
		}},
	}))
	spec := domain.FileSpec{
		Name:   "produce.txt",
		Kind:   domain.FileKindStatic,
		Source: domain.FileSource{Type: "inline", Content: ""},
		Processors: []domain.ProcessorSpec{{
			Type:  "script",
			Stage: domain.StageFile,
			Params: params(t, map[string]any{
				"source": inlineScriptSource(`function main(input, api) {
  var nodes = api.subscription.produce("sub");
  var rendered = api.subscription.produce("sub", {target: "json-nodes", args: {prefix: "api-"}});
  var contentNodes = JSON.parse(rendered.content);
  input.file.content = nodes.kind + ":" + nodes.nodes[0].name + "\n" + rendered.kind + ":" + rendered.target + ":" + contentNodes[0].name;
  return input;
}`),
			}),
		}},
	}

	result, err := svc.GetFile(ctx, domain.FileRequest{
		Spec:    &spec,
		Request: domain.RequestInfo{Args: map[string]string{"prefix": "outer-"}},
	})

	require.NoError(t, err)
	require.Equal(t, "nodes:node-a\ncontent:json-nodes:api-node-a", string(result.File.Content))
	require.Contains(t, result.Report.Dependencies, domain.ResourceRef{Kind: "subscription", Name: "sub"})
}

func TestServiceSubscriptionPreviewReceivesRequestArgs(t *testing.T) {
	ctx := context.Background()
	svc := service.New(service.WithFS(afero.NewMemMapFs()))
	require.NoError(t, svc.PutSubscription(ctx, domain.Subscription{
		Name:    "sub",
		Type:    domain.SubscriptionTypeLocal,
		Format:  "uri-list",
		Content: "ss://aes-128-gcm:secret@example.com:8388#node-a",
		Processors: []domain.ProcessorSpec{{
			Type:  "script",
			Stage: domain.StageNodes,
			Params: params(t, map[string]any{
				"source": inlineScriptSource(`function main(input) {
  var prefix = input.args.prefix || "";
  input.nodes.forEach(function(node) { node.name = prefix + node.name; });
  return input;
}`),
			}),
		}},
	}))

	preview, err := svc.PreviewSubscription(ctx, "sub", map[string]string{"prefix": "api-"})

	require.NoError(t, err)
	require.Equal(t, "node-a", preview.Nodes[0].Before.Name)
	require.Equal(t, "api-node-a", preview.Nodes[0].After.Name)
}
