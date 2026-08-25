// Package script executes sandboxed JavaScript processors for Sandrone workflows.
package script

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dop251/goja"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

// Config is the runtime configuration for a script. It is parsed from
// ProcessorSpec.Params and validated when the processor is constructed.
type Config struct {
	Path        string           `json:"path,omitempty" jsonschema:"Legacy controlled file source name"`
	Content     string           `json:"content,omitempty" jsonschema:"Legacy inline JavaScript source"`
	Source      ScriptSource     `json:"source,omitempty" jsonschema:"Structured inline controlled file or controlled remote source"`
	Engine      string           `json:"engine,omitempty" jsonschema:"Script engine" enum:"js" default:"js"`
	Args        map[string]any   `json:"args,omitempty" jsonschema:"Arguments exposed to the script envelope"`
	TimeoutMS   int              `json:"timeout_ms,omitempty" jsonschema:"Execution timeout override in milliseconds; zero inherits the project script default" minimum:"0"`
	ScriptID    string           `json:"id,omitempty" jsonschema:"Stable identifier for inline source diagnostics"`
	Permissions PermissionConfig `json:"permissions,omitempty" jsonschema:"Explicit side-effect permissions"`
}

type ScriptSource struct {
	Type    string              `json:"type,omitempty" jsonschema:"Script source kind" enum:"inline,file,remote"`
	Content string              `json:"content,omitempty" jsonschema:"JavaScript body for inline source"`
	Name    string              `json:"name,omitempty" jsonschema:"Controlled resource name for file source"`
	Remote  *domain.RemoteInput `json:"remote,omitempty" jsonschema:"Controlled remote source configuration"`
	SHA256  string              `json:"sha256,omitempty" jsonschema:"Optional lowercase SHA-256 integrity digest" pattern:"^[0-9a-f]{64}$"`
}

// PermissionConfig declares which side-effect APIs the script may use.
// The default is all-off: network and store access remain disabled until
// explicitly enabled in the spec.
type PermissionConfig struct {
	Network   bool     `json:"network,omitempty" jsonschema:"Allow controlled remote reads through injected APIs"`
	Resources []string `json:"resources,omitempty" jsonschema:"Controlled subscription and file resources the script may read"`
}

type Loader func(ctx context.Context, source ScriptSource) (content string, id string, err error)

const defaultTimeout = 2 * time.Second

// loadScriptContent resolves Config.Source into the script body and a stable
// identifier used by the JS compiler and source trace.
func loadScriptContent(ctx context.Context, cfg Config, loader Loader) (string, string, error) {
	if cfg.Source.Type == "inline" {
		id := cfg.ScriptID
		if id == "" {
			id = "<inline>"
		}
		return cfg.Source.Content, id, nil
	}
	if cfg.Source.Type == "" {
		return "", "", &domain.AppError{
			Code:    domain.CodeProcessorConfigInvalid,
			Message: "script requires exactly one source",
		}
	}
	if loader == nil {
		return "", "", &domain.AppError{
			Code:    domain.CodeProcessorConfigInvalid,
			Message: "script source loading is not configured",
		}
	}
	body, id, err := loader(ctx, cfg.Source)
	if err != nil {
		if domain.IsCode(err, domain.CodeFileDependencyCycle) {
			return "", "", err
		}
		return "", "", &domain.AppError{
			Code:    domain.CodeProcessorConfigInvalid,
			Message: fmt.Sprintf("load script %s", scriptSourceID(cfg.Source)),
			Cause:   err,
		}
	}
	if id == "" {
		id = scriptSourceID(cfg.Source)
	}
	return body, id, nil
}

func scriptSourceID(source ScriptSource) string {
	switch source.Type {
	case "file":
		return source.Name
	case "remote":
		if source.Remote != nil {
			return source.Remote.URL
		}
	}
	return source.Type
}

// runner compiles the script once and runs main(input, api) on each
// invocation. goja runtimes are not safe for concurrent use, so each
// invocation grabs the mutex.
type runner struct {
	mu      sync.Mutex
	id      string
	program *goja.Program
	cfg     Config
	api     *scriptAPI
	loader  Loader
}

func newRunner(cfg Config, api *scriptAPI, loader Loader) (*runner, error) {
	r := &runner{cfg: cfg, api: api, loader: loader}
	if cfg.Source.Type != "inline" {
		if loader == nil {
			return nil, &domain.AppError{
				Code:    domain.CodeProcessorConfigInvalid,
				Message: "script source loading is not configured",
			}
		}
		return r, nil
	}
	content, id, err := loadScriptContent(context.Background(), cfg, loader)
	if err != nil {
		return nil, err
	}
	r.id = id
	r.program, err = compileScript(content, id)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (r *runner) run(ctx context.Context, envelope ScriptEnvelope) (ScriptEnvelope, error) {
	program, id, err := r.programForRun(ctx)
	if err != nil {
		return envelope, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.resetAPISinks()

	timeout := defaultTimeout
	if r.cfg.TimeoutMS > 0 {
		timeout = time.Duration(r.cfg.TimeoutMS) * time.Millisecond
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if r.api != nil {
		r.api.begin(runCtx, envelope)
		defer r.api.end()
	}

	vm := goja.New()
	vm.SetFieldNameMapper(goja.TagFieldNameMapper("json", true))
	r.api.attach(vm)
	nodeLineageSymbol := goja.NewSymbol("sandrone.node.lineage")

	done := make(chan struct{})
	var timedOut atomic.Bool
	isScriptTimeout := func() bool {
		return timedOut.Load() || (runCtx.Err() == context.DeadlineExceeded && ctx.Err() == nil)
	}
	go func() {
		select {
		case <-runCtx.Done():
			err := runCtx.Err()
			if err == context.DeadlineExceeded && ctx.Err() == nil {
				timedOut.Store(true)
				vm.Interrupt("script_timeout")
				return
			}
			vm.Interrupt(err.Error())
		case <-done:
		}
	}()
	defer close(done)

	if _, err := vm.RunProgram(program); err != nil {
		if isScriptTimeout() {
			return envelope, &domain.AppError{
				Code:    domain.CodeScriptTimeout,
				Message: fmt.Sprintf("script %s timed out", id),
			}
		}
		return envelope, &domain.AppError{
			Code:    domain.CodeScriptRuntime,
			Message: fmt.Sprintf("eval script %s", id),
			Cause:   err,
		}
	}

	mainFn, ok := goja.AssertFunction(vm.Get("main"))
	if !ok {
		return envelope, &domain.AppError{
			Code:    domain.CodeScriptRuntime,
			Message: fmt.Sprintf("script %s missing main(input, api)", id),
		}
	}

	inputObj, err := envelopeToJSValue(vm, envelope, r.cfg.Args, nodeLineageSymbol)
	if err != nil {
		return envelope, err
	}

	result, err := mainFn(goja.Undefined(), inputObj, vm.Get("api"))
	if err != nil {
		if isScriptTimeout() {
			return envelope, &domain.AppError{
				Code:    domain.CodeScriptTimeout,
				Message: fmt.Sprintf("script %s timed out", id),
			}
		}
		var appErr *domain.AppError
		if errors.As(err, &appErr) {
			return envelope, appErr
		}
		return envelope, &domain.AppError{
			Code:    domain.CodeScriptRuntime,
			Message: fmt.Sprintf("script %s main() failed", id),
			Cause:   err,
		}
	}
	if result == nil || goja.IsUndefined(result) || goja.IsNull(result) {
		return r.appendAPIWarnings(envelope), nil
	}
	updated, err := jsValueToEnvelope(result, envelope, nodeLineageSymbol)
	if err != nil {
		return envelope, err
	}
	return r.appendAPIWarnings(updated), nil
}

func (r *runner) programForRun(ctx context.Context) (*goja.Program, string, error) {
	if r.cfg.Source.Type == "inline" {
		return r.program, r.id, nil
	}
	content, id, err := loadScriptContent(ctx, r.cfg, r.loader)
	if err != nil {
		return nil, "", err
	}
	program, err := compileScript(content, id)
	if err != nil {
		return nil, "", err
	}
	return program, id, nil
}

func compileScript(content, id string) (*goja.Program, error) {
	program, err := goja.Compile(id, content, true)
	if err != nil {
		return nil, &domain.AppError{
			Code:    domain.CodeScriptRuntime,
			Message: fmt.Sprintf("compile script %s", id),
			Cause:   err,
		}
	}
	return program, nil
}

func (r *runner) resetAPISinks() {
	if r.api == nil {
		return
	}
	if r.api.warningSink != nil {
		*r.api.warningSink = nil
	}
	if r.api.logSink != nil {
		*r.api.logSink = nil
	}
}

func (r *runner) appendAPIWarnings(envelope ScriptEnvelope) ScriptEnvelope {
	if r.api == nil || r.api.warningSink == nil || len(*r.api.warningSink) == 0 {
		return envelope
	}
	envelope.Warnings = append(envelope.Warnings, (*r.api.warningSink)...)
	return envelope
}

func envelopeToJSValue(vm *goja.Runtime, envelope ScriptEnvelope, args map[string]any, nodeLineageSymbol *goja.Symbol) (goja.Value, error) {
	body, err := json.Marshal(envelope)
	if err != nil {
		return nil, &domain.AppError{
			Code:    domain.CodeScriptRuntime,
			Message: "encode script envelope",
			Cause:   err,
		}
	}
	value, err := jsonParseInVM(vm, body)
	if err != nil {
		return nil, &domain.AppError{
			Code:    domain.CodeScriptRuntime,
			Message: "decode script envelope",
			Cause:   err,
		}
	}
	mergedArgs := mergeScriptArgs(args, envelope.Args)
	if mergedArgs != nil {
		obj, ok := value.(*goja.Object)
		if !ok {
			return nil, &domain.AppError{
				Code:    domain.CodeScriptRuntime,
				Message: "envelope is not a JS object",
			}
		}
		argsBody, err := json.Marshal(mergedArgs)
		if err != nil {
			return nil, err
		}
		argsValue, err := jsonParseInVM(vm, argsBody)
		if err != nil {
			return nil, err
		}
		if err := obj.Set("args", argsValue); err != nil {
			return nil, err
		}
	}
	if err := attachNodeLineageSymbols(vm, value, envelope.Nodes, nodeLineageSymbol); err != nil {
		return nil, err
	}
	return value, nil
}

func attachNodeLineageSymbols(vm *goja.Runtime, value goja.Value, nodes []ScriptNode, symbol *goja.Symbol) error {
	if len(nodes) == 0 || symbol == nil {
		return nil
	}
	root, ok := value.(*goja.Object)
	if !ok {
		return &domain.AppError{Code: domain.CodeScriptRuntime, Message: "envelope is not a JS object"}
	}
	jsNodes, ok := root.Get("nodes").(*goja.Object)
	if !ok {
		return &domain.AppError{Code: domain.CodeScriptRuntime, Message: "envelope nodes is not a JS array"}
	}
	for index, node := range nodes {
		if node.lineage == "" {
			continue
		}
		jsNode, ok := jsNodes.Get(strconv.Itoa(index)).(*goja.Object)
		if !ok {
			return &domain.AppError{Code: domain.CodeScriptRuntime, Message: fmt.Sprintf("envelope node %d is not a JS object", index)}
		}
		if err := jsNode.DefineDataPropertySymbol(symbol, vm.ToValue(node.lineage), goja.FLAG_FALSE, goja.FLAG_FALSE, goja.FLAG_TRUE); err != nil {
			return &domain.AppError{Code: domain.CodeScriptRuntime, Message: fmt.Sprintf("attach lineage to envelope node %d", index), Cause: err}
		}
	}
	return nil
}

func mergeScriptArgs(configArgs map[string]any, requestArgs map[string]any) map[string]any {
	if len(configArgs) == 0 && len(requestArgs) == 0 {
		return nil
	}
	out := make(map[string]any, len(configArgs)+len(requestArgs))
	for key, value := range configArgs {
		out[key] = value
	}
	for key, value := range requestArgs {
		out[key] = value
	}
	return out
}

// jsonParseInVM runs the runtime's native JSON.parse on the provided body so
// scripts receive idiomatic JS objects rather than Go-backed proxies.
// Go-backed proxies don't propagate mutations on nested arrays, which breaks
// scripts that mutate the envelope in place.
func jsonParseInVM(vm *goja.Runtime, body []byte) (goja.Value, error) {
	jsonObj, ok := vm.Get("JSON").(*goja.Object)
	if !ok {
		return nil, &domain.AppError{Code: domain.CodeScriptRuntime, Message: "JSON global missing"}
	}
	parse, ok := goja.AssertFunction(jsonObj.Get("parse"))
	if !ok {
		return nil, &domain.AppError{Code: domain.CodeScriptRuntime, Message: "JSON.parse missing"}
	}
	return parse(goja.Undefined(), vm.ToValue(string(body)))
}

func jsValueToEnvelope(value goja.Value, fallback ScriptEnvelope, nodeLineageSymbol *goja.Symbol) (ScriptEnvelope, error) {
	exported := value.Export()
	body, err := json.Marshal(exported)
	if err != nil {
		return fallback, &domain.AppError{
			Code:    domain.CodeScriptRuntime,
			Message: "encode script return value",
			Cause:   err,
		}
	}
	var updated ScriptEnvelope
	if err := json.Unmarshal(body, &updated); err != nil {
		return fallback, &domain.AppError{
			Code:    domain.CodeScriptRuntime,
			Message: "decode script return value",
			Cause:   err,
		}
	}
	restoreNodeLineageSymbols(value, updated.Nodes, nodeLineageSymbol)
	return updated, nil
}

func restoreNodeLineageSymbols(value goja.Value, nodes []ScriptNode, symbol *goja.Symbol) {
	if len(nodes) == 0 || symbol == nil {
		return
	}
	root, ok := value.(*goja.Object)
	if !ok {
		return
	}
	jsNodes, ok := root.Get("nodes").(*goja.Object)
	if !ok {
		return
	}
	for index := range nodes {
		jsNode, ok := jsNodes.Get(strconv.Itoa(index)).(*goja.Object)
		if !ok {
			continue
		}
		lineageValue := jsNode.GetSymbol(symbol)
		if lineageValue == nil || goja.IsUndefined(lineageValue) || goja.IsNull(lineageValue) {
			continue
		}
		lineage, ok := lineageValue.Export().(string)
		if ok {
			nodes[index].lineage = lineage
		}
	}
}

// scriptAPI is the controlled object exposed as the second argument to
// main(input, api). Only the side-effect-free helpers are enabled by
// default; loadResource / loadFile / produce / http require explicit
// permission and are currently reserved.
