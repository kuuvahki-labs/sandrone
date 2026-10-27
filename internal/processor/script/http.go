package script

import (
	"fmt"

	"github.com/dop251/goja"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
)

func (a *scriptAPI) jsHTTP(vm *goja.Runtime) goja.Value {
	obj := vm.NewObject()
	must(obj.Set("request", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) < 2 || goja.IsNull(call.Argument(0)) || goja.IsNull(call.Argument(1)) {
			panic(vm.NewGoError(domain.NewError(domain.CodeScriptRuntime, "api.http.request requires nodes and options")))
		}
		var nodes []ScriptNode
		if err := exportJSValueStrict(call.Argument(0), &nodes); err != nil {
			panic(vm.NewGoError(fmt.Errorf("decode api.http.request nodes: %w", err)))
		}
		allowed, err := scriptProbeNodesAllowed(a.probeNodeCounts, nodes)
		if err != nil {
			panic(vm.NewGoError(err))
		}
		if !allowed {
			panic(vm.NewGoError(domain.NewError(domain.CodeScriptRuntime, "api.http.request nodes must be a subset of the script input nodes")))
		}
		var options domain.NodeHTTPOptions
		if err := exportJSValueStrict(call.Argument(1), &options); err != nil {
			panic(vm.NewGoError(fmt.Errorf("decode api.http.request options: %w", err)))
		}
		converted, warnings, err := scriptToNodes(nodes)
		if err != nil {
			panic(vm.NewGoError(err))
		}
		if a.warningSink != nil {
			*a.warningSink = append(*a.warningSink, warnings...)
		}
		results, err := a.httpRequester.RequestNodeHTTP(a.ctx, converted, options)
		if err != nil {
			panic(vm.NewGoError(err))
		}
		if results == nil {
			results = []domain.NodeHTTPResult{}
		}
		return jsJSONValue(vm, results)
	}))
	return obj
}
