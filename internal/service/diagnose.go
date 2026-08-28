package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/kuuvahki-labs/sandrone/internal/domain"
	"github.com/kuuvahki-labs/sandrone/internal/nodevalidation"
	"github.com/kuuvahki-labs/sandrone/internal/processor"
)

type diagnosedLocalInput struct {
	kind         domain.DiagnoseInputKind
	format       string
	subscription *domain.Subscription
	file         *domain.FileSpec
}

// Diagnose identifies an input and faithfully executes its declared pipeline.
// Expected diagnostic failures are returned as a structured failed result;
// request-shape errors are returned as Go errors.
func (s *Service) Diagnose(ctx context.Context, req domain.DiagnoseRequest) (*domain.DiagnoseResult, error) {
	if req.Kind == "" {
		req.Kind = domain.DiagnoseInputAuto
	}
	if req.CacheMode == "" {
		req.CacheMode = domain.DiagnoseCacheModeRefresh
	}
	if err := validateDiagnoseRequest(req); err != nil {
		return nil, err
	}
	recorder := processor.NewTraceRecorder()
	ctx = processor.WithTrace(ctx, recorder)
	result := &domain.DiagnoseResult{
		Status: domain.DiagnoseStatusFailed,
		Input:  domain.DiagnoseInput{Kind: req.Kind, Name: req.Name},
		Stages: []domain.DiagnoseStage{},
	}

	var err error
	switch {
	case req.Remote != nil:
		result.Input = domain.DiagnoseInput{Kind: domain.DiagnoseInputNodes, Name: req.Name, Format: normalizeFormat(req.Format), Remote: true}
		err = s.diagnoseNodes(ctx, req, result)
	case req.SubscriptionName != "":
		result.Input = domain.DiagnoseInput{Kind: domain.DiagnoseInputSubscription, Name: req.SubscriptionName}
		err = s.diagnoseStoredSubscription(ctx, req, result)
	case req.File != nil:
		result.Input = domain.DiagnoseInput{Kind: domain.DiagnoseInputFile, Name: firstNonEmptyString(req.File.Name, req.Name)}
		fileReq := *req.File
		fileReq.Refresh = req.CacheMode != domain.DiagnoseCacheModeReuse
		err = s.diagnoseFile(ctx, fileReq, result)
	default:
		detected, detectErr := s.detectDiagnoseInput(ctx, req)
		if detectErr != nil {
			err = detectErr
			break
		}
		result.Input = domain.DiagnoseInput{Kind: detected.kind, Name: req.Name, Format: detected.format}
		if len(req.Processors) > 0 && detected.kind != domain.DiagnoseInputNodes {
			err = domain.NewError(domain.CodeInvalidArgument, "one-time processors are only supported for nodes input")
			break
		}
		if strings.TrimSpace(req.Format) != "" && detected.kind != domain.DiagnoseInputNodes {
			err = domain.NewError(domain.CodeInvalidArgument, "format override is only supported for nodes input")
			break
		}
		switch detected.kind {
		case domain.DiagnoseInputNodes:
			req.Kind = domain.DiagnoseInputNodes
			req.Format = detected.format
			err = s.diagnoseNodes(ctx, req, result)
		case domain.DiagnoseInputSubscription:
			err = s.diagnoseSubscription(ctx, *detected.subscription, req, result)
		case domain.DiagnoseInputFile:
			err = s.diagnoseFile(ctx, domain.FileRequest{
				Spec: detected.file, Target: req.Target, Meta: req.Meta,
				Refresh: req.CacheMode != domain.DiagnoseCacheModeReuse,
			}, result)
		}
	}

	processorStages := recorder.Snapshot()
	for i := range processorStages {
		processorStages[i].Index = len(result.Stages) + i + 1
	}
	result.Stages = append(result.Stages, processorStages...)
	if err != nil {
		result.Error = diagnoseAppError(err)
		result.Status = domain.DiagnoseStatusFailed
		if result.Report.Kind == "" {
			result.Report = s.prepareReport("diagnose", domain.Report{
				Warnings:     append([]domain.Warning{}, result.Warnings...),
				Dependencies: append([]domain.ResourceRef{}, result.Dependencies...),
				SourceRefs:   append([]domain.SourceRef{}, result.SourceRefs...),
			})
		}
		result.Report.Status = string(domain.DiagnoseStatusFailed)
		return result, nil
	}
	result.Status = diagnoseStatus(result)
	result.Report.Status = string(result.Status)
	return result, nil
}

func validateDiagnoseRequest(req domain.DiagnoseRequest) error {
	sources := 0
	if req.Remote != nil {
		sources++
	}
	if req.SubscriptionName != "" {
		sources++
	}
	if req.File != nil {
		sources++
	}
	if len(req.Content) > 0 || sources == 0 {
		sources++
	}
	if sources != 1 {
		return domain.NewError(domain.CodeInvalidArgument, "diagnose request must identify exactly one input")
	}
	switch req.Kind {
	case domain.DiagnoseInputAuto, domain.DiagnoseInputNodes, domain.DiagnoseInputSubscription, domain.DiagnoseInputFile:
	default:
		return domain.NewError(domain.CodeInvalidArgument, "diagnose kind must be auto, nodes, subscription, or file")
	}
	switch req.CacheMode {
	case domain.DiagnoseCacheModeRefresh, domain.DiagnoseCacheModeReuse:
	default:
		return domain.NewError(domain.CodeInvalidArgument, "diagnose cache_mode must be refresh or reuse")
	}
	if req.Remote != nil && req.Kind != domain.DiagnoseInputAuto && req.Kind != domain.DiagnoseInputNodes {
		return domain.NewError(domain.CodeInvalidArgument, "remote diagnose input only supports nodes")
	}
	if req.SubscriptionName != "" && req.Kind != domain.DiagnoseInputAuto && req.Kind != domain.DiagnoseInputSubscription {
		return domain.NewError(domain.CodeInvalidArgument, "subscription diagnose input requires subscription kind")
	}
	if req.File != nil && req.Kind != domain.DiagnoseInputAuto && req.Kind != domain.DiagnoseInputFile {
		return domain.NewError(domain.CodeInvalidArgument, "file diagnose input requires file kind")
	}
	if (req.SubscriptionName != "" || req.File != nil) && (len(req.Processors) > 0 || strings.TrimSpace(req.Format) != "") {
		return domain.NewError(domain.CodeInvalidArgument, "format and one-time processors are only supported for nodes input")
	}
	if len(req.Processors) > 0 && req.Kind != domain.DiagnoseInputAuto && req.Kind != domain.DiagnoseInputNodes {
		return domain.NewError(domain.CodeInvalidArgument, "one-time processors are only supported for nodes input")
	}
	return nil
}

func (s *Service) detectDiagnoseInput(ctx context.Context, req domain.DiagnoseRequest) (*diagnosedLocalInput, error) {
	if req.Kind == domain.DiagnoseInputAuto && !isAutoNodeFormat(req.Format) {
		format, err := s.detectNodeFormat(ctx, req.Content, req.Format)
		if err != nil {
			return nil, err
		}
		return &diagnosedLocalInput{kind: domain.DiagnoseInputNodes, format: format}, nil
	}
	if req.Kind == domain.DiagnoseInputNodes {
		format, err := s.detectNodeFormat(ctx, req.Content, req.Format)
		if err != nil {
			return nil, err
		}
		return &diagnosedLocalInput{kind: req.Kind, format: format}, nil
	}
	if req.Kind == domain.DiagnoseInputSubscription {
		sub, err := decodeSubscriptionDefinition(req.Content)
		if err != nil {
			return nil, err
		}
		return &diagnosedLocalInput{kind: req.Kind, subscription: sub}, nil
	}
	if req.Kind == domain.DiagnoseInputFile {
		spec, err := decodeFileSpecDefinition(req.Content)
		if err != nil {
			return nil, err
		}
		return &diagnosedLocalInput{kind: req.Kind, file: spec}, nil
	}

	candidates := []diagnosedLocalInput{}
	doc, _ := decodeTopLevelDocument(req.Content)
	if looksLikeFileSpec(doc) {
		if spec, err := decodeFileSpecDefinition(req.Content); err == nil {
			candidates = append(candidates, diagnosedLocalInput{kind: domain.DiagnoseInputFile, file: spec})
		}
	}
	if looksLikeSubscription(doc) {
		if sub, err := decodeSubscriptionDefinition(req.Content); err == nil {
			candidates = append(candidates, diagnosedLocalInput{kind: domain.DiagnoseInputSubscription, subscription: sub})
		}
	}
	for _, format := range s.detectStrongNodeFormats(ctx, req.Content, doc) {
		candidates = append(candidates, diagnosedLocalInput{kind: domain.DiagnoseInputNodes, format: format})
	}
	if len(candidates) == 0 {
		return nil, domain.NewError(domain.CodeInputKindUnrecognized, "input is not a recognized node document, Subscription, or FileSpec")
	}
	if len(candidates) > 1 {
		labels := make([]string, 0, len(candidates))
		for _, candidate := range candidates {
			label := string(candidate.kind)
			if candidate.format != "" {
				label += "/" + candidate.format
			}
			labels = append(labels, label)
		}
		return nil, domain.NewError(domain.CodeInputKindAmbiguous, "input matches multiple kinds: "+strings.Join(labels, ", "))
	}
	return &candidates[0], nil
}

func (s *Service) detectNodeFormat(ctx context.Context, content []byte, override string) (string, error) {
	if !isAutoNodeFormat(override) {
		if _, ok := s.parsers[normalizeFormat(override)]; !ok {
			return "", domain.NewError(domain.CodeInvalidArgument, fmt.Sprintf("unsupported parse format %q", override))
		}
		return normalizeFormat(override), nil
	}
	doc, _ := decodeTopLevelDocument(content)
	candidates := s.detectStrongNodeFormats(ctx, content, doc)
	if len(candidates) == 0 {
		return "", domain.NewError(domain.CodeInputKindUnrecognized, "input is not a recognized node document")
	}
	if len(candidates) > 1 {
		return "", domain.NewError(domain.CodeInputKindAmbiguous, "node input matches multiple formats: "+strings.Join(candidates, ", "))
	}
	// Ensure the strong candidate is accepted by its parser before execution.
	if _, err := s.parseNodeContentExplicit(ctx, candidates[0], content, nil); err != nil {
		return candidates[0], err
	}
	return candidates[0], nil
}

func (s *Service) detectStrongNodeFormats(ctx context.Context, content []byte, doc map[string]any) []string {
	formats := []string{}
	if hasAnyTopLevelKey(doc, "outbounds", "endpoints") {
		formats = append(formats, "sing-box")
	}
	if hasAnyTopLevelKey(doc, "proxies") {
		formats = append(formats, "mihomo")
	}
	trimmed := bytes.TrimSpace(content)
	if len(trimmed) > 0 && trimmed[0] == '[' {
		var nodes []domain.NodeIR
		if json.Unmarshal(trimmed, &nodes) == nil {
			formats = append(formats, "json-nodes")
		}
	} else if hasAnyTopLevelKey(doc, "nodes") && !looksLikeSubscription(doc) {
		formats = append(formats, "json-nodes")
	}
	if decoded, ok := decodeAutoSubscriptionBase64(content); ok && s.looksLikeStrongURIList(ctx, decoded) {
		formats = append(formats, "base64")
	} else if s.looksLikeStrongURIList(ctx, content) {
		count := 0
		for line := range strings.SplitSeq(string(content), "\n") {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "#") {
				count++
			}
		}
		if count == 1 {
			formats = append(formats, "uri")
		} else {
			formats = append(formats, "uri-list")
		}
	}
	return formats
}

func (s *Service) looksLikeStrongURIList(ctx context.Context, content []byte) bool {
	nodes, _, err := s.uriParser.ParseStrictList(ctx, content)
	return err == nil && len(nodes) > 0
}

func looksLikeFileSpec(doc map[string]any) bool {
	if doc == nil {
		return false
	}
	kind, _ := doc["kind"].(string)
	switch domain.FileKind(strings.TrimSpace(kind)) {
	case domain.FileKindStatic, domain.FileKindMihomo, domain.FileKindSingBox, domain.FileKindShadowrocket:
		return true
	default:
		return false
	}
}

func looksLikeSubscription(doc map[string]any) bool {
	if doc == nil {
		return false
	}
	name, nameOK := doc["name"].(string)
	typeName, typeOK := doc["type"].(string)
	if !nameOK || strings.TrimSpace(name) == "" || !typeOK {
		return false
	}
	switch domain.SubscriptionType(strings.ToLower(strings.TrimSpace(typeName))) {
	case domain.SubscriptionTypeRemote:
		remote, ok := doc["remote"].(map[string]any)
		return ok && strings.TrimSpace(fmt.Sprint(remote["url"])) != ""
	case domain.SubscriptionTypeLocal:
		_, content := doc["content"]
		return content
	case domain.SubscriptionTypeCollection:
		_, inputs := doc["inputs"]
		_, nodes := doc["nodes"]
		return inputs || nodes
	default:
		return false
	}
}

func decodeSubscriptionDefinition(content []byte) (*domain.Subscription, error) {
	var sub domain.Subscription
	if err := decodeJSONDefinition(content, &sub); err != nil {
		return nil, domain.WrapError(domain.CodeParseFailed, "decode Subscription", err)
	}
	return &sub, nil
}

func decodeFileSpecDefinition(content []byte) (*domain.FileSpec, error) {
	var spec domain.FileSpec
	if err := decodeJSONDefinition(content, &spec); err != nil {
		return nil, domain.WrapError(domain.CodeParseFailed, "decode FileSpec", err)
	}
	return &spec, nil
}

func decodeJSONDefinition(content []byte, out any) error {
	trimmed := bytes.TrimSpace(content)
	if len(trimmed) == 0 {
		return errors.New("empty input")
	}
	if trimmed[0] != '{' {
		return errors.New("resource definitions must use a JSON object")
	}
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.UseNumber()
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values are not supported")
		}
		return err
	}
	return nil
}

func (s *Service) diagnoseNodes(ctx context.Context, req domain.DiagnoseRequest, result *domain.DiagnoseResult) error {
	var (
		parsed *parseInputResult
		err    error
	)
	if req.Remote != nil {
		remoteInput, fetchErr := s.fetchRemoteInput(ctx, *req.Remote)
		if fetchErr != nil {
			return fetchErr
		}
		format := req.Format
		if isAutoNodeFormat(format) {
			format, err = s.detectNodeFormat(ctx, remoteInput.Body, "")
			if err != nil {
				return err
			}
		}
		result.Input.Format = normalizeFormat(format)
		parsed, err = s.parseNodeContent(ctx, format, remoteInput.Body, false, &remoteInput.SourceRef)
	} else {
		parsed, err = s.parseRequestInput(ctx, domain.ParseRequest{Format: req.Format, Content: req.Content})
	}
	if err != nil {
		return err
	}
	if parsed.Source != nil {
		if result.Input.Format == "" {
			result.Input.Format = parsed.Source.Format
		}
		result.SourceRefs = append(result.SourceRefs, parsed.Source.SourceRefs...)
		result.Warnings = append(result.Warnings, parsed.Source.Warnings...)
	}
	for _, node := range parsed.Nodes {
		result.Warnings = append(result.Warnings, node.Warnings...)
	}
	prepared, prepareErr := prepareNodeBatch(parsed.Nodes)
	if prepareErr != nil {
		return domain.WrapError(domain.CodeInvalidArgument, "assign node runtime identity", prepareErr)
	}
	validated := nodevalidation.Validate(prepared, nodevalidation.StageNormalized, req.Target)
	result.Counts = validated.Counts
	result.Issues = append(result.Issues, validated.Issues...)
	result.Warnings = append(result.Warnings, validationDropWarnings(parsed.Nodes, validated.Issues, nodevalidation.StageNormalized, req.Target)...)
	result.Stages = append(result.Stages, domain.DiagnoseStage{Index: 1, Scope: "input", Kind: "parse", Type: result.Input.Format, InputCount: len(parsed.Nodes), OutputCount: len(validated.Nodes), Warnings: append([]domain.Warning{}, result.Warnings...)})
	if validated.Counts.Input > 0 && validated.Counts.Valid == 0 {
		return domain.NewError(domain.CodeNodeValidationFailed, fmt.Sprintf("all %d node(s) failed semantic validation", validated.Counts.Input))
	}
	nodes := validated.Nodes
	if len(req.Processors) > 0 {
		processed, processErr := s.registry.RunNodes(processor.WithTraceScope(ctx, "input"), req.Processors, domain.NodeProcessInput{
			Target: req.Target, Nodes: nodes,
			Context: domain.NodeContext{InputName: req.Name, Sources: sourcesSlice(parsed.Source)},
			Request: domain.RequestInfo{Meta: req.Meta},
		})
		if processErr != nil {
			return processErr
		}
		result.Warnings = append(result.Warnings, processed.Warnings...)
		postPrepared, prepareErr := prepareNodeBatch(processed.Nodes)
		if prepareErr != nil {
			return domain.WrapError(domain.CodeInvalidArgument, "assign node runtime identity", prepareErr)
		}
		post := nodevalidation.Validate(postPrepared, nodevalidation.StageProcessed, req.Target)
		result.Issues = append(result.Issues, post.Issues...)
		result.Counts.Valid = post.Counts.Valid
		result.Counts.Invalid += post.Counts.Invalid
		result.Counts.Errors += post.Counts.Errors
		result.Counts.Warnings += post.Counts.Warnings
		result.Warnings = append(result.Warnings, validationDropWarnings(processed.Nodes, post.Issues, nodevalidation.StageProcessed, req.Target)...)
		if post.Counts.Input > 0 && post.Counts.Valid == 0 {
			return domain.NewError(domain.CodeNodeValidationFailed, fmt.Sprintf("all %d node(s) failed semantic validation after processors", post.Counts.Input))
		}
		nodes = post.Nodes
	}
	if len(nodes) == 0 {
		return domain.NewError(domain.CodeNodeValidationFailed, "node input produced no final nodes")
	}
	result.Nodes = append([]domain.NodeIR{}, nodes...)
	result.Report = s.prepareReport("diagnose", domain.Report{Warnings: append([]domain.Warning{}, result.Warnings...), SourceRefs: append([]domain.SourceRef{}, result.SourceRefs...)})
	return nil
}

func (s *Service) diagnoseStoredSubscription(ctx context.Context, req domain.DiagnoseRequest, result *domain.DiagnoseResult) error {
	if s.metaStore == nil {
		return storeUnavailable()
	}
	sub, err := s.metaStore.GetSubscription(ctx, req.SubscriptionName)
	if err != nil {
		return err
	}
	return s.diagnoseSubscription(withSubscriptionCacheOwner(ctx, sub.Name), sub, req, result)
}

func (s *Service) diagnoseSubscription(ctx context.Context, sub domain.Subscription, req domain.DiagnoseRequest, result *domain.DiagnoseResult) error {
	execution, err := s.executeSubscription(ctx, sub, subscriptionExecutionRequest{
		Name: sub.Name, Request: domain.RequestInfo{Meta: req.Meta},
		Refresh: req.CacheMode != domain.DiagnoseCacheModeReuse,
	}, newSubscriptionExecutionState())
	if err != nil {
		return err
	}
	set := execution.After
	result.Input.Kind = domain.DiagnoseInputSubscription
	result.Input.Name = sub.Name
	result.Input.Format = normalizeFormat(sub.Format)
	if isAutoNodeFormat(result.Input.Format) {
		result.Input.Format = diagnosedSourceFormat(set.Sources)
	}
	result.Nodes = append([]domain.NodeIR{}, set.Nodes...)
	result.Warnings = append(result.Warnings, set.Warnings...)
	result.Dependencies = append(result.Dependencies, set.Dependencies...)
	result.SourceRefs = append(result.SourceRefs, sourceRefsFromSources(set.Sources)...)
	result.Counts = domain.ValidationCounts{Input: len(set.Nodes), Valid: len(set.Nodes)}
	result.Stages = append(result.Stages, domain.DiagnoseStage{Index: 1, Scope: "subscription:" + firstNonEmptyString(sub.Name, "inline"), Kind: "materialize", InputCount: len(set.Nodes), OutputCount: len(set.Nodes), Warnings: append([]domain.Warning{}, set.Warnings...)})
	result.Report = s.prepareReport("diagnose", domain.Report{Warnings: append([]domain.Warning{}, result.Warnings...), Dependencies: append([]domain.ResourceRef{}, result.Dependencies...), SourceRefs: append([]domain.SourceRef{}, result.SourceRefs...)})
	if len(set.Nodes) == 0 {
		return domain.NewError(domain.CodeNodeValidationFailed, "subscription produced no nodes")
	}
	return nil
}

func diagnosedSourceFormat(sources []domain.SourceInfo) string {
	format := ""
	for _, source := range sources {
		candidate := normalizeFormat(source.Format)
		if candidate == "" {
			continue
		}
		if format == "" {
			format = candidate
			continue
		}
		if format != candidate {
			return "mixed"
		}
	}
	return format
}

func (s *Service) diagnoseFile(ctx context.Context, req domain.FileRequest, result *domain.DiagnoseResult) error {
	fileResult, err := s.GetFile(ctx, req)
	if err != nil {
		return err
	}
	doc := fileResult.File
	result.File = &doc
	result.Warnings = append(result.Warnings, fileResult.Report.Warnings...)
	result.Dependencies = append(result.Dependencies, fileResult.Report.Dependencies...)
	result.SourceRefs = append(result.SourceRefs, fileResult.Report.SourceRefs...)
	result.Report = s.prepareReport("diagnose", fileResult.Report)
	result.Stages = append(result.Stages, domain.DiagnoseStage{Index: 1, Scope: "file:" + firstNonEmptyString(doc.Name, req.Name, "inline"), Kind: "compile", InputCount: len(doc.Parts), OutputCount: len(doc.Parts), Warnings: append([]domain.Warning{}, fileResult.Report.Warnings...)})
	return nil
}

func diagnoseStatus(result *domain.DiagnoseResult) domain.DiagnoseStatus {
	partial := len(result.Warnings) > 0 || len(result.Issues) > 0 || result.Counts.Invalid > 0
	for _, stage := range result.Stages {
		if stage.Error != nil || stage.OutputCount < stage.InputCount || len(stage.Warnings) > 0 {
			partial = true
		}
		for _, probeResult := range stage.Probes {
			for _, nodeResult := range probeResult.Results {
				if !nodeResult.Alive {
					partial = true
				}
			}
		}
	}
	if partial {
		return domain.DiagnoseStatusPartial
	}
	return domain.DiagnoseStatusOK
}

func diagnoseAppError(err error) *domain.AppError {
	if appErr, ok := errors.AsType[*domain.AppError](err); ok {
		cloned := *appErr
		cloned.Cause = nil
		return &cloned
	}
	if errors.Is(err, os.ErrNotExist) {
		return &domain.AppError{Code: domain.CodeFileInputNotFound, Message: err.Error()}
	}
	return &domain.AppError{Code: domain.CodeInvalidArgument, Message: err.Error()}
}
