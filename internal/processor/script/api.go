package script

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dop251/goja"
	"gopkg.in/yaml.v3"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

type scriptAPI struct {
	cfg              Config
	warningSink      *[]domain.Warning
	logSink          *[]string
	probeRunner      ProbeRunner
	resourceResolver ResourceResolver
	ctx              context.Context
	stage            string
	probeNodeCounts  map[string]int
}

func newScriptAPI(cfg Config, warningSink *[]domain.Warning, logSink *[]string, probeRunner ProbeRunner, resolver ResourceResolver) *scriptAPI {
	return &scriptAPI{cfg: cfg, warningSink: warningSink, logSink: logSink, probeRunner: probeRunner, resourceResolver: resolver}
}

func (a *scriptAPI) begin(ctx context.Context, envelope ScriptEnvelope) {
	a.ctx = ctx
	a.stage = envelope.Stage
	a.probeNodeCounts = scriptProbeNodeCounts(envelope.Nodes)
}

func (a *scriptAPI) end() {
	a.ctx = nil
	a.stage = ""
	a.probeNodeCounts = nil
}

func (a *scriptAPI) attach(vm *goja.Runtime) {
	obj := vm.NewObject()
	must(obj.Set("log", a.jsLog(vm)))
	must(obj.Set("warn", a.jsWarn()))
	must(obj.Set("yaml", a.jsYAML(vm)))
	must(obj.Set("json", a.jsJSON(vm)))
	must(obj.Set("base64", a.jsBase64(vm)))
	must(obj.Set("hash", a.jsHash(vm)))
	if a.resourceResolver != nil {
		must(obj.Set("subscription", a.jsSubscription(vm)))
		must(obj.Set("file", a.jsFile(vm)))
	}
	if a.probeRunner != nil && a.stage == string(domain.StageNodes) {
		must(obj.Set("probe", a.jsProbe(vm)))
	}
	must(vm.Set("api", obj))
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func (a *scriptAPI) jsLog(vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if a.logSink == nil {
			return goja.Undefined()
		}
		parts := make([]string, 0, len(call.Arguments))
		for _, arg := range call.Arguments {
			parts = append(parts, fmt.Sprintf("%v", arg.Export()))
		}
		*a.logSink = append(*a.logSink, fmt.Sprint(parts))
		_ = vm
		return goja.Undefined()
	}
}

func (a *scriptAPI) jsWarn() func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if a.warningSink == nil || len(call.Arguments) == 0 {
			return goja.Undefined()
		}
		raw := call.Argument(0).Export()
		body, err := json.Marshal(raw)
		if err != nil {
			return goja.Undefined()
		}
		var w domain.Warning
		if err := json.Unmarshal(body, &w); err == nil {
			if w.Code == "" {
				w.Code = "script_warning"
			}
			*a.warningSink = append(*a.warningSink, w)
		}
		return goja.Undefined()
	}
}

type scriptProbeOptions struct {
	Layer           string            `json:"layer,omitempty"`
	Method          string            `json:"method,omitempty"`
	Core            string            `json:"core,omitempty"`
	URL             string            `json:"url,omitempty"`
	NTPServer       string            `json:"ntp_server,omitempty"`
	ExpectedStatus  string            `json:"expected_status,omitempty"`
	TimeoutMS       int               `json:"timeout_ms,omitempty"`
	Attempts        int               `json:"attempts,omitempty"`
	Concurrency     int               `json:"concurrency,omitempty"`
	CacheTTLSeconds int               `json:"cache_ttl_seconds,omitempty"`
	Meta            map[string]string `json:"meta,omitempty"`
}

func (a *scriptAPI) jsProbe(vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if a.probeRunner == nil {
			panic(vm.NewGoError(&domain.AppError{Code: domain.CodeScriptRuntime, Message: "api.probe is not available"}))
		}
		if len(call.Arguments) == 0 {
			panic(vm.NewGoError(&domain.AppError{Code: domain.CodeScriptRuntime, Message: "api.probe requires nodes"}))
		}

		var scriptNodes []ScriptNode
		if err := exportJSValue(call.Argument(0), &scriptNodes); err != nil {
			panic(vm.NewGoError(fmt.Errorf("decode api.probe nodes: %w", err)))
		}
		if !scriptProbeNodesAllowed(a.probeNodeCounts, scriptNodes) {
			panic(vm.NewGoError(&domain.AppError{Code: domain.CodeScriptRuntime, Message: "api.probe nodes must be a subset of the script input nodes"}))
		}
		nodes, warnings, err := scriptToNodes(scriptNodes)
		if err != nil {
			panic(vm.NewGoError(fmt.Errorf("convert api.probe nodes: %w", err)))
		}
		if a.warningSink != nil {
			*a.warningSink = append(*a.warningSink, warnings...)
		}

		var options scriptProbeOptions
		if len(call.Arguments) > 1 && !goja.IsUndefined(call.Argument(1)) && !goja.IsNull(call.Argument(1)) {
			if err := exportJSValue(call.Argument(1), &options); err != nil {
				panic(vm.NewGoError(fmt.Errorf("decode api.probe options: %w", err)))
			}
		}
		req := domain.ProbeRequest{
			Input: domain.NodeInput{
				Name:  "script",
				Type:  "inline_nodes",
				Nodes: nodes,
				Meta:  cloneStringMap(options.Meta),
			},
			Layer:           domain.ProbeLayer(options.Layer),
			Method:          domain.ProbeMethod(options.Method),
			Core:            options.Core,
			URL:             options.URL,
			NTPServer:       options.NTPServer,
			ExpectedStatus:  options.ExpectedStatus,
			TimeoutMS:       options.TimeoutMS,
			Attempts:        options.Attempts,
			Concurrency:     options.Concurrency,
			CacheTTLSeconds: options.CacheTTLSeconds,
			Meta:            cloneStringMap(options.Meta),
		}
		ctx := a.ctx
		if ctx == nil {
			ctx = context.Background()
		}
		result, err := a.probeRunner.Probe(ctx, req)
		if err != nil {
			panic(vm.NewGoError(err))
		}
		if result == nil {
			panic(vm.NewGoError(&domain.AppError{Code: domain.CodeScriptRuntime, Message: "api.probe returned nil result"}))
		}
		if a.warningSink != nil {
			*a.warningSink = append(*a.warningSink, result.Report.Warnings...)
		}
		body, err := json.Marshal(result)
		if err != nil {
			panic(vm.NewGoError(err))
		}
		value, err := jsonParseInVM(vm, body)
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return value
	}
}

func (a *scriptAPI) jsSubscription(vm *goja.Runtime) goja.Value {
	obj := vm.NewObject()
	must(obj.Set("produce", func(call goja.FunctionCall) goja.Value {
		if a.resourceResolver == nil {
			panic(vm.NewGoError(&domain.AppError{Code: domain.CodeScriptRuntime, Message: "api.subscription.produce is not available"}))
		}
		if len(call.Arguments) == 0 || strings.TrimSpace(call.Argument(0).String()) == "" {
			panic(vm.NewGoError(&domain.AppError{Code: domain.CodeInvalidArgument, Message: "api.subscription.produce requires a subscription name"}))
		}
		opts, err := scriptProduceOptionsFromCall(call, 1)
		if err != nil {
			panic(vm.NewGoError(fmt.Errorf("decode api.subscription.produce options: %w", err)))
		}
		ctx := a.ctx
		if ctx == nil {
			ctx = context.Background()
		}
		result, err := a.resourceResolver.ProduceSubscription(ctx, call.Argument(0).String(), opts)
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return jsJSONValue(vm, result)
	}))
	return obj
}

func (a *scriptAPI) jsFile(vm *goja.Runtime) goja.Value {
	obj := vm.NewObject()
	must(obj.Set("content", func(call goja.FunctionCall) goja.Value {
		if a.resourceResolver == nil {
			panic(vm.NewGoError(&domain.AppError{Code: domain.CodeScriptRuntime, Message: "api.file.content is not available"}))
		}
		if len(call.Arguments) == 0 || strings.TrimSpace(call.Argument(0).String()) == "" {
			panic(vm.NewGoError(&domain.AppError{Code: domain.CodeInvalidArgument, Message: "api.file.content requires a file name"}))
		}
		opts, err := scriptProduceOptionsFromCall(call, 1)
		if err != nil {
			panic(vm.NewGoError(fmt.Errorf("decode api.file.content options: %w", err)))
		}
		ctx := a.ctx
		if ctx == nil {
			ctx = context.Background()
		}
		content, err := a.resourceResolver.FileContent(ctx, call.Argument(0).String(), opts)
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(content)
	}))
	return obj
}

func scriptProduceOptionsFromCall(call goja.FunctionCall, index int) (domain.ScriptProduceOptions, error) {
	var opts domain.ScriptProduceOptions
	if len(call.Arguments) <= index || goja.IsUndefined(call.Argument(index)) || goja.IsNull(call.Argument(index)) {
		return opts, nil
	}
	if err := exportJSValue(call.Argument(index), &opts); err != nil {
		return opts, err
	}
	return opts, nil
}

func jsJSONValue(vm *goja.Runtime, value any) goja.Value {
	body, err := json.Marshal(value)
	if err != nil {
		panic(vm.NewGoError(err))
	}
	result, err := jsonParseInVM(vm, body)
	if err != nil {
		panic(vm.NewGoError(err))
	}
	return result
}

func exportJSValue(value goja.Value, out any) error {
	body, err := json.Marshal(value.Export())
	if err != nil {
		return err
	}
	return json.Unmarshal(body, out)
}

func scriptProbeNodeCounts(nodes []ScriptNode) map[string]int {
	out := make(map[string]int, len(nodes))
	for _, node := range nodes {
		out[scriptProbeNodeKey(node)]++
	}
	return out
}

func scriptProbeNodesAllowed(allowed map[string]int, nodes []ScriptNode) bool {
	remaining := make(map[string]int, len(allowed))
	for key, count := range allowed {
		remaining[key] = count
	}
	for _, node := range nodes {
		key := scriptProbeNodeKey(node)
		if remaining[key] <= 0 {
			return false
		}
		remaining[key]--
	}
	return true
}

func scriptProbeNodeKey(node ScriptNode) string {
	body, _ := json.Marshal(node) //nolint:gosec // ScriptNode credentials are compared in memory to enforce api.probe subset checks.
	return string(body)
}

func (a *scriptAPI) jsYAML(vm *goja.Runtime) goja.Value {
	obj := vm.NewObject()
	must(obj.Set("parse", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			return goja.Undefined()
		}
		var v any
		if err := yaml.Unmarshal([]byte(call.Argument(0).String()), &v); err != nil {
			panic(vm.NewGoError(err))
		}
		body, err := json.Marshal(normalizeForJS(v))
		if err != nil {
			panic(vm.NewGoError(err))
		}
		result, err := jsonParseInVM(vm, body)
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return result
	}))
	must(obj.Set("stringify", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			return vm.ToValue("")
		}
		body, err := yaml.Marshal(call.Argument(0).Export())
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(string(body))
	}))
	return obj
}

func (a *scriptAPI) jsJSON(vm *goja.Runtime) goja.Value {
	obj := vm.NewObject()
	must(obj.Set("parse", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			return goja.Undefined()
		}
		result, err := jsonParseInVM(vm, []byte(call.Argument(0).String()))
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return result
	}))
	must(obj.Set("stringify", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			return vm.ToValue("")
		}
		body, err := json.Marshal(call.Argument(0).Export())
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(string(body))
	}))
	return obj
}

func (a *scriptAPI) jsBase64(vm *goja.Runtime) goja.Value {
	obj := vm.NewObject()
	must(obj.Set("encode", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			return vm.ToValue("")
		}
		return vm.ToValue(base64.StdEncoding.EncodeToString([]byte(call.Argument(0).String())))
	}))
	must(obj.Set("decode", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			return vm.ToValue("")
		}
		body, err := base64.StdEncoding.DecodeString(call.Argument(0).String())
		if err != nil {
			panic(vm.NewGoError(err))
		}
		return vm.ToValue(string(body))
	}))
	return obj
}

func (a *scriptAPI) jsHash(vm *goja.Runtime) goja.Value {
	obj := vm.NewObject()
	must(obj.Set("sha256", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			return vm.ToValue("")
		}
		return vm.ToValue(sha256Hex(call.Argument(0).String()))
	}))
	return obj
}

// normalizeForJS converts yaml.v3's map[interface{}]interface{} into
// map[string]any so goja can index it with string keys.
func normalizeForJS(v any) any {
	switch m := v.(type) {
	case map[any]any:
		out := make(map[string]any, len(m))
		for k, val := range m {
			out[fmt.Sprint(k)] = normalizeForJS(val)
		}
		return out
	case []any:
		out := make([]any, len(m))
		for i, val := range m {
			out[i] = normalizeForJS(val)
		}
		return out
	default:
		return v
	}
}
