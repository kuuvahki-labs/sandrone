// Package agentcatalog provides catalog schema and metadata generation for processors,
// files, and subscriptions.
package agentcatalog

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/inidoc"
	"github.com/kuuvahki-labs/sandrone/internal/processor"
	scriptproc "github.com/kuuvahki-labs/sandrone/internal/processor/script"
	"github.com/kuuvahki-labs/sandrone/internal/service"
)

type ProcessorCatalogDocument struct {
	Type         string             `json:"type"`
	Stage        domain.Stage       `json:"stage"`
	Description  string             `json:"description"`
	ParamsSchema *jsonschema.Schema `json:"params_schema"`
	Effects      processor.Effects  `json:"effects"`
	Examples     []map[string]any   `json:"examples"`
	ErrorCodes   []domain.ErrorCode `json:"error_codes"`
}

type FileKindCatalogDocument struct {
	Kind              domain.FileKind             `json:"kind"`
	Description       string                      `json:"description"`
	SettingsSupported bool                        `json:"settings_supported"`
	SettingsSchema    *jsonschema.Schema          `json:"settings_schema,omitempty"`
	MediaType         string                      `json:"media_type"`
	Syntax            string                      `json:"syntax"`
	DefaultExtension  string                      `json:"default_extension"`
	SourceRules       service.FileKindSourceRules `json:"source_rules"`
	Defaults          map[string]any              `json:"defaults"`
	Examples          []map[string]any            `json:"examples"`
}

type ScriptAPIDocument struct {
	Version      int                               `json:"version"`
	ConfigSchema *jsonschema.Schema                `json:"config_schema"`
	Envelopes    map[string]ScriptEnvelopeDocument `json:"envelopes"`
	Methods      []ScriptMethodDocument            `json:"methods"`
	Sources      []ScriptSourceDocument            `json:"sources"`
	Sandbox      ScriptSandboxDocument             `json:"sandbox"`
}

type ScriptEnvelopeDocument struct {
	InputSchema  *jsonschema.Schema `json:"input_schema"`
	OutputSchema *jsonschema.Schema `json:"output_schema"`
}

type ScriptMethodDocument struct {
	Name                  string                   `json:"name"`
	Description           string                   `json:"description"`
	Stages                []domain.Stage           `json:"stages"`
	RuntimeRequirement    string                   `json:"runtime_requirement,omitempty"`
	Arguments             []ScriptArgumentDocument `json:"arguments"`
	RecommendedArity      string                   `json:"recommended_arity"`
	ExtraArgumentsIgnored bool                     `json:"extra_arguments_ignored"`
	ZeroArguments         string                   `json:"zero_arguments"`
	Returns               ScriptReturnDocument     `json:"returns"`
	ErrorCodes            []domain.ErrorCode       `json:"error_codes"`
}

type ScriptArgumentDocument struct {
	Position int                `json:"position"`
	Name     string             `json:"name"`
	Schema   *jsonschema.Schema `json:"schema"`
	Required bool               `json:"required"`
	Variadic bool               `json:"variadic,omitempty"`
}

type ScriptReturnDocument struct {
	Kind   string             `json:"kind"`
	Schema *jsonschema.Schema `json:"schema,omitempty"`
}

type ScriptSourceDocument struct {
	Type        string `json:"type"`
	Controlled  bool   `json:"controlled"`
	Description string `json:"description"`
}

type ScriptSandboxDocument struct {
	Filesystem bool   `json:"filesystem"`
	Process    bool   `json:"process"`
	Network    string `json:"network"`
	Timeout    string `json:"timeout"`
	Logging    string `json:"logging"`
	Sensitive  string `json:"sensitive_data"`
}

type scriptProbeOptions struct {
	Method          string            `json:"method,omitempty" enum:"tcp_connect,udp_ntp,url_test"`
	Core            string            `json:"core,omitempty"`
	URL             string            `json:"url,omitempty"`
	NTPServer       string            `json:"ntp_server,omitempty"`
	ExpectedStatus  string            `json:"expected_status,omitempty"`
	TimeoutMS       int               `json:"timeout_ms,omitempty" minimum:"0"`
	Attempts        int               `json:"attempts,omitempty" minimum:"0"`
	Concurrency     int               `json:"concurrency,omitempty" minimum:"0"`
	CacheTTLSeconds int               `json:"cache_ttl_seconds,omitempty" minimum:"0"`
	Meta            map[string]string `json:"meta,omitempty"`
}

func ProcessorDetail(descriptor processor.Descriptor) (ProcessorCatalogDocument, error) {
	schema, err := schemaForPrototype(descriptor.ParamsPrototype)
	if err != nil {
		return ProcessorCatalogDocument{}, fmt.Errorf("processor %s/%s params schema: %w", descriptor.Stage, descriptor.Type, err)
	}
	return ProcessorCatalogDocument{
		Type: descriptor.Type, Stage: descriptor.Stage, Description: descriptor.Description,
		ParamsSchema: schema, Effects: descriptor.Effects, Examples: descriptor.Examples,
		ErrorCodes: descriptor.ErrorCodes,
	}, nil
}

func FileKindDetail(capability service.FileKindCapability) (FileKindCatalogDocument, error) {
	document := FileKindCatalogDocument{
		Kind: capability.Kind, Description: capability.Description,
		MediaType: capability.MediaType, Syntax: capability.Syntax,
		DefaultExtension: capability.DefaultExtension, SourceRules: capability.SourceRules,
		Defaults: capability.Defaults, Examples: capability.Examples,
	}
	if capability.SettingsPrototype == nil {
		return document, nil
	}
	schema, err := fileSettingsSchemaForPrototype(capability.SettingsPrototype)
	if err != nil {
		return FileKindCatalogDocument{}, fmt.Errorf("file kind %s settings schema: %w", capability.Kind, err)
	}
	document.SettingsSupported = true
	document.SettingsSchema = schema
	return document, nil
}

func ScriptAPI() (ScriptAPIDocument, error) {
	configSchema, err := schemaForPrototype(scriptproc.Config{})
	if err != nil {
		return ScriptAPIDocument{}, fmt.Errorf("script config schema: %w", err)
	}
	nodesEnvelope, err := scriptEnvelopeSchema(domain.StageNodes)
	if err != nil {
		return ScriptAPIDocument{}, err
	}
	fileEnvelope, err := scriptEnvelopeSchema(domain.StageFile)
	if err != nil {
		return ScriptAPIDocument{}, err
	}
	methods, err := scriptMethodCatalog()
	if err != nil {
		return ScriptAPIDocument{}, err
	}
	return ScriptAPIDocument{
		Version:      1,
		ConfigSchema: configSchema,
		Envelopes: map[string]ScriptEnvelopeDocument{
			string(domain.StageNodes): {InputSchema: nodesEnvelope.CloneSchemas(), OutputSchema: nodesEnvelope.CloneSchemas()},
			string(domain.StageFile):  {InputSchema: fileEnvelope.CloneSchemas(), OutputSchema: fileEnvelope.CloneSchemas()},
		},
		Methods: methods,
		Sources: []ScriptSourceDocument{
			{Type: "inline", Controlled: true, Description: "Inline JavaScript supplied in processor configuration."},
			{Type: "file", Controlled: true, Description: "A named file resource resolved by the service."},
			{Type: "remote", Controlled: true, Description: "An HTTP(S) source fetched through the controlled service fetcher."},
		},
		Sandbox: ScriptSandboxDocument{
			Filesystem: false,
			Process:    false,
			Network:    "Only injected controlled remote reads are available when explicitly permitted.",
			Timeout:    "Execution is bounded by timeout_ms and the service default.",
			Logging:    "api.log output stays in the processor diagnostic sink.",
			Sensitive:  "Scripts receive only the selected processor envelope and explicitly allowed resources.",
		},
	}, nil
}

func scriptEnvelopeSchema(stage domain.Stage) (*jsonschema.Schema, error) {
	schema, err := schemaForPrototype(scriptproc.ScriptEnvelope{})
	if err != nil {
		return nil, fmt.Errorf("script %s envelope schema: %w", stage, err)
	}
	stageValue := any(string(stage))
	versionValue := any(float64(1))
	schema.Properties["stage"].Const = &stageValue
	schema.Properties["version"].Const = &versionValue
	return schema, nil
}

func scriptMethodCatalog() ([]ScriptMethodDocument, error) {
	type argumentSpec struct {
		name      string
		prototype any
		anyJSON   bool
		nullable  bool
		required  bool
		variadic  bool
	}
	type methodSpec struct {
		name, description     string
		stages                []domain.Stage
		runtimeRequirement    string
		arguments             []argumentSpec
		recommendedArity      string
		extraArgumentsIgnored bool
		zeroArguments         string
		returnKind            string
		result                any
		resultAnyJSON         bool
		codes                 []domain.ErrorCode
	}
	allStages := []domain.Stage{domain.StageNodes, domain.StageFile}
	noErrors := []domain.ErrorCode{}
	runtimeOnly := []domain.ErrorCode{domain.CodeScriptRuntime}
	specs := []methodSpec{
		{
			name: "api.log", description: "Append diagnostic values to the script log sink.", stages: allStages,
			arguments:        []argumentSpec{{name: "values", anyJSON: true, variadic: true}},
			recommendedArity: "0+", zeroArguments: "returns void",
			returnKind: "void", codes: noErrors,
		},
		{
			name: "api.warn", description: "Append a structured warning to the output envelope.", stages: allStages,
			arguments:        []argumentSpec{{name: "warning", anyJSON: true}},
			recommendedArity: "0-1", extraArgumentsIgnored: true, zeroArguments: "returns void without adding a warning",
			returnKind: "void", codes: noErrors,
		},
		{
			name: "api.yaml.parse", description: "Parse YAML into JSON-shaped data.", stages: allStages,
			arguments:        []argumentSpec{{name: "value", anyJSON: true}},
			recommendedArity: "0-1", extraArgumentsIgnored: true, zeroArguments: "returns void",
			returnKind: "value_or_void", resultAnyJSON: true, codes: runtimeOnly,
		},
		{
			name: "api.yaml.stringify", description: "Encode JSON-shaped data as YAML.", stages: allStages,
			arguments:        []argumentSpec{{name: "value", anyJSON: true}},
			recommendedArity: "0-1", extraArgumentsIgnored: true, zeroArguments: "returns empty string",
			returnKind: "value", result: "", codes: runtimeOnly,
		},
		{
			name: "api.json.parse", description: "Parse JSON into JavaScript data.", stages: allStages,
			arguments:        []argumentSpec{{name: "value", anyJSON: true}},
			recommendedArity: "0-1", extraArgumentsIgnored: true, zeroArguments: "returns void",
			returnKind: "value_or_void", resultAnyJSON: true, codes: runtimeOnly,
		},
		{
			name: "api.json.stringify", description: "Encode JavaScript data as JSON.", stages: allStages,
			arguments:        []argumentSpec{{name: "value", anyJSON: true}},
			recommendedArity: "0-1", extraArgumentsIgnored: true, zeroArguments: "returns empty string",
			returnKind: "value", result: "", codes: runtimeOnly,
		},
		{
			name: "api.ini.parse", description: "Parse INI into an ordered document that preserves duplicate sections and raw body lines.", stages: allStages,
			arguments:        []argumentSpec{{name: "value", prototype: ""}},
			recommendedArity: "0-1", extraArgumentsIgnored: true, zeroArguments: "returns void",
			returnKind: "value_or_void", result: inidoc.Model{}, codes: runtimeOnly,
		},
		{
			name: "api.ini.stringify", description: "Validate and encode an ordered INI document using canonical section headers.", stages: allStages,
			arguments:        []argumentSpec{{name: "document", prototype: inidoc.Model{}}},
			recommendedArity: "0-1", extraArgumentsIgnored: true, zeroArguments: "returns empty string",
			returnKind: "value", result: "", codes: runtimeOnly,
		},
		{
			name: "api.ini.override", description: "Apply an ordered, lossless INI section override patch.", stages: allStages,
			arguments: []argumentSpec{
				{name: "base", prototype: "", required: true},
				{name: "patch", prototype: "", required: true},
			},
			recommendedArity: "2", extraArgumentsIgnored: true, zeroArguments: "returns script_runtime error",
			returnKind: "value", result: "", codes: runtimeOnly,
		},
		{
			name: "api.base64.encode", description: "Encode UTF-8 text as standard base64.", stages: allStages,
			arguments:        []argumentSpec{{name: "value", anyJSON: true}},
			recommendedArity: "0-1", extraArgumentsIgnored: true, zeroArguments: "returns empty string",
			returnKind: "value", result: "", codes: noErrors,
		},
		{
			name: "api.base64.decode", description: "Decode standard base64 into UTF-8 text.", stages: allStages,
			arguments:        []argumentSpec{{name: "value", anyJSON: true}},
			recommendedArity: "0-1", extraArgumentsIgnored: true, zeroArguments: "returns empty string",
			returnKind: "value", result: "", codes: runtimeOnly,
		},
		{
			name: "api.hash.sha256", description: "Hash UTF-8 text with SHA-256.", stages: allStages,
			arguments:        []argumentSpec{{name: "value", anyJSON: true}},
			recommendedArity: "0-1", extraArgumentsIgnored: true, zeroArguments: "returns empty string",
			returnKind: "value", result: "", codes: noErrors,
		},
		{
			name: "api.subscription.produce", description: "Produce a named subscription through the service.",
			stages: allStages,
			arguments: []argumentSpec{
				{name: "name", prototype: "", required: true},
				{name: "options", prototype: domain.ScriptProduceOptions{}, nullable: true},
			},
			recommendedArity: "1-2", extraArgumentsIgnored: true, zeroArguments: "returns invalid_argument error",
			returnKind: "value", result: domain.ScriptSubscriptionProduceResult{},
			codes: []domain.ErrorCode{domain.CodeInvalidArgument, domain.CodeScriptRuntime},
		},
		{
			name: "api.file.content", description: "Read the rendered content of an allowed named file through the service.",
			stages:             allStages,
			runtimeRequirement: "The function is injected at nodes and file stages, but succeeds only during file rendering.",
			arguments: []argumentSpec{
				{name: "name", prototype: "", required: true},
				{name: "options", prototype: domain.ScriptProduceOptions{}, nullable: true},
			},
			recommendedArity: "1-2", extraArgumentsIgnored: true, zeroArguments: "returns invalid_argument error",
			returnKind: "value", result: "",
			codes: []domain.ErrorCode{domain.CodeInvalidArgument, domain.CodeFileDependencyCycle, domain.CodeScriptRuntime},
		},
		{
			name: "api.probe", description: "Probe a subset of the node-stage input through the service.",
			stages: []domain.Stage{domain.StageNodes},
			arguments: []argumentSpec{
				{name: "nodes", prototype: []scriptproc.ScriptNode{}, required: true},
				{name: "options", prototype: scriptProbeOptions{}, nullable: true},
			},
			recommendedArity: "1-2", extraArgumentsIgnored: true, zeroArguments: "returns script_runtime error",
			returnKind: "value", result: domain.ProbeResult{},
			codes: []domain.ErrorCode{
				domain.CodeInvalidArgument, domain.CodeProbeBackendUnavailable,
				domain.CodeProbeCoreUnavailable, domain.CodeProbeCoreStartFailed,
				domain.CodeProbeCoreAPIFailed, domain.CodeProbeInvalidTarget,
				domain.CodeProbeTimeout, domain.CodeProbeTCPFailed,
				domain.CodeProbeUDPNTPFailed, domain.CodeScriptRuntime,
			},
		},
	}
	methods := make([]ScriptMethodDocument, len(specs))
	for index, spec := range specs {
		arguments := make([]ScriptArgumentDocument, len(spec.arguments))
		for argumentIndex, argument := range spec.arguments {
			schema := &jsonschema.Schema{}
			var err error
			if !argument.anyJSON {
				schema, err = schemaForPrototype(argument.prototype)
				if err != nil {
					return nil, fmt.Errorf("%s argument %s schema: %w", spec.name, argument.name, err)
				}
			}
			if argument.nullable {
				schema = allowExplicitNull(schema)
			}
			arguments[argumentIndex] = ScriptArgumentDocument{
				Position: argumentIndex, Name: argument.name, Schema: schema,
				Required: argument.required, Variadic: argument.variadic,
			}
		}
		returns := ScriptReturnDocument{Kind: spec.returnKind}
		if spec.returnKind != "void" {
			returns.Schema = &jsonschema.Schema{}
			if !spec.resultAnyJSON {
				result, err := schemaForPrototype(spec.result)
				if err != nil {
					return nil, fmt.Errorf("%s result schema: %w", spec.name, err)
				}
				returns.Schema = result
			}
		}
		methods[index] = ScriptMethodDocument{
			Name: spec.name, Description: spec.description, Stages: spec.stages,
			RuntimeRequirement: spec.runtimeRequirement, Arguments: arguments,
			RecommendedArity: spec.recommendedArity, ExtraArgumentsIgnored: spec.extraArgumentsIgnored,
			ZeroArguments: spec.zeroArguments, Returns: returns, ErrorCodes: spec.codes,
		}
	}
	return methods, nil
}

func allowExplicitNull(schema *jsonschema.Schema) *jsonschema.Schema {
	schema = schema.CloneSchemas()
	switch {
	case schema.Type != "":
		schema.Types = []string{schema.Type, "null"}
		schema.Type = ""
	case len(schema.Types) > 0:
		for _, schemaType := range schema.Types {
			if schemaType == "null" {
				return schema
			}
		}
		schema.Types = append(schema.Types, "null")
	default:
		nullValue := any(nil)
		schema.AnyOf = append(schema.AnyOf, &jsonschema.Schema{Const: &nullValue})
	}
	return schema
}

func schemaForPrototype(prototype any) (*jsonschema.Schema, error) {
	if prototype == nil {
		return nil, fmt.Errorf("prototype is required")
	}
	prototypeType := reflect.TypeOf(prototype)
	schema, err := jsonschema.ForType(prototypeType, &jsonschema.ForOptions{
		TypeSchemas: map[reflect.Type]*jsonschema.Schema{
			reflect.TypeOf(json.RawMessage{}): {},
		},
	})
	if err != nil {
		return nil, err
	}
	if err := projectConstraintTags(prototypeType, schema); err != nil {
		return nil, err
	}
	return schema, nil
}

func fileSettingsSchemaForPrototype(prototype any) (*jsonschema.Schema, error) {
	schema, err := schemaForPrototype(prototype)
	if err != nil {
		return nil, err
	}
	stripReflectedNullTypes(reflect.TypeOf(prototype), schema)
	return schema, nil
}

func stripReflectedNullTypes(valueType reflect.Type, schema *jsonschema.Schema) {
	if schema == nil {
		return
	}
	if len(schema.Types) > 0 {
		types := make([]string, 0, len(schema.Types))
		for _, schemaType := range schema.Types {
			if schemaType != "null" {
				types = append(types, schemaType)
			}
		}
		switch len(types) {
		case 1:
			schema.Type = types[0]
			schema.Types = nil
		default:
			schema.Types = types
		}
	}
	for valueType.Kind() == reflect.Pointer {
		valueType = valueType.Elem()
	}
	switch valueType.Kind() {
	case reflect.Struct:
		for _, field := range reflect.VisibleFields(valueType) {
			if field.Anonymous {
				continue
			}
			name := strings.Split(field.Tag.Get("json"), ",")[0]
			if name == "" {
				name = field.Name
			}
			if name == "-" {
				continue
			}
			stripReflectedNullTypes(field.Type, schema.Properties[name])
		}
	case reflect.Slice, reflect.Array:
		stripReflectedNullTypes(valueType.Elem(), schema.Items)
	case reflect.Map:
		stripReflectedNullTypes(valueType.Elem(), schema.AdditionalProperties)
	}
}

func projectConstraintTags(valueType reflect.Type, schema *jsonschema.Schema) error {
	for valueType.Kind() == reflect.Pointer {
		valueType = valueType.Elem()
	}
	switch valueType.Kind() {
	case reflect.Struct:
		for _, field := range reflect.VisibleFields(valueType) {
			if field.Anonymous {
				continue
			}
			name := strings.Split(field.Tag.Get("json"), ",")[0]
			if name == "" {
				name = field.Name
			}
			if name == "-" {
				continue
			}
			property := schema.Properties[name]
			if property == nil {
				continue
			}
			if err := applyConstraintTags(field, property); err != nil {
				return fmt.Errorf("%s.%s: %w", valueType, field.Name, err)
			}
			if err := projectConstraintTags(field.Type, property); err != nil {
				return err
			}
		}
	case reflect.Slice, reflect.Array:
		if schema.Items != nil {
			return projectConstraintTags(valueType.Elem(), schema.Items)
		}
	}
	return nil
}

func applyConstraintTags(field reflect.StructField, schema *jsonschema.Schema) error {
	if value := field.Tag.Get("enum"); value != "" {
		parts := strings.Split(value, ",")
		schema.Enum = make([]any, len(parts))
		for index, part := range parts {
			schema.Enum[index] = part
		}
	}
	if value := field.Tag.Get("minimum"); value != "" {
		number, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("minimum: %w", err)
		}
		schema.Minimum = &number
	}
	if value := field.Tag.Get("maximum"); value != "" {
		number, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("maximum: %w", err)
		}
		schema.Maximum = &number
	}
	if value := field.Tag.Get("default"); value != "" {
		raw := json.RawMessage(value)
		if !json.Valid(raw) {
			raw, _ = json.Marshal(value)
		}
		schema.Default = raw
	}
	if value := field.Tag.Get("pattern"); value != "" {
		schema.Pattern = value
	}
	if value := field.Tag.Get("minItems"); value != "" {
		count, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("minItems: %w", err)
		}
		schema.MinItems = &count
	}
	return nil
}

func ProcessorSchemaURI(stage domain.Stage, processorType string) string {
	return "sandrone://schemas/processors/" + string(stage) + "/" + processorType
}
