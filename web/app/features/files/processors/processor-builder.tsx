import { useCallback, useState } from "react";
import Alert from "@mui/material/Alert";

import type { FileDriverDefinition, FileMergeMode } from "~/features/files/drivers/core/file-driver";
import { requireFileDriver } from "~/features/files/drivers/registry";
import type { FileKind } from "~/features/files/model/types";
import { type Translator, useI18n } from "~/shared/i18n/context";
import {
  KeyValueParamsEditor,
  ProcessorEditorList,
  type ProcessorParamsEditorProps,
} from "~/shared/processors/components/processor-editor-list";
import {
  defaultScriptParams,
  sanitizeScriptParams,
  ScriptProcessorParamsEditor,
} from "~/shared/processors/components/script-processor-params-editor";
import {
  cleanParams,
  createProcessorDraftId,
  customProcessorName,
  type ProcessorDraft,
  processorLabel as modelProcessorLabel,
  stringValue,
} from "~/shared/processors/model";
import type { ProcessorDetail, ResourceOption } from "~/shared/resources/types";

import { FileMergeParamsEditor } from "./merge-params-editor";
import {
  recognizeRuleSourceRewriteProcessorPreset,
  RULE_SOURCE_REWRITE_PRESET_OPTION,
  ruleSourceRewriteProcessorPreset,
} from "./rule-source-rewrite-preset";

const RULE_SOURCE_REWRITE_KINDS = new Set(["mihomo", "sing-box", "shadowrocket"]);

export function FileProcessorBuilder({ defaultValue = [], kind, onDirty, onValidityChange, scriptFiles = [] }: { defaultValue?: ProcessorDetail[]; kind: FileKind; onDirty?: () => void; onValidityChange?: (valid: boolean) => void; scriptFiles?: ResourceOption[] }) {
  const { t } = useI18n();
  const [validationIssueCount, setValidationIssueCount] = useState(0);
  const driver = requireFileDriver(kind);
  const processorOptions = [
    { value: "script", label: t("model.processor.script") },
    ...(driver.processors.mergeModes.length ? [{ value: "merge", label: t("model.processor.merge") }] : []),
    ...(RULE_SOURCE_REWRITE_KINDS.has(kind) ? [{
      value: RULE_SOURCE_REWRITE_PRESET_OPTION,
      label: t("files.processor.ruleSourceRewritePreset"),
    }] : []),
    ...(driver.processors.adapter?.options?.(t) ?? []),
  ];

  function ParamsEditor(props: ProcessorParamsEditorProps) {
    return <FileProcessorParamsEditor {...props} kind={kind} scriptFiles={scriptFiles} />;
  }

  const serializeProcessorDraft = useCallback(
    (draft: ProcessorDraft) => serializeDraft(draft, t, kind),
    [kind, t],
  );
  const handleValueChange = useCallback((value: ProcessorDetail[]) => {
    const issues = driver.processors.validate(value);
    setValidationIssueCount(issues.length);
    onValidityChange?.(issues.length === 0);
  }, [driver, onValidityChange]);

  return (
    <div className="grid gap-3">
      <ProcessorEditorList
        addProcessorDrafts={(type, current) => addFileProcessorDrafts(type, current, kind, driver)}
        createDraftId={createProcessorID}
        defaultParams={(type) => defaultParams(type, kind)}
        defaultType="script"
        defaultValue={driver.processors.adapter?.normalize(defaultValue) ?? defaultValue}
        draftProcessors={draftProcessors}
        paramsEditor={ParamsEditor}
        processorOptions={processorOptions}
        serializeDraft={serializeProcessorDraft}
        onDirty={onDirty}
        onValueChange={handleValueChange}
      />
      {validationIssueCount > 0 ? <Alert severity="error">{t("processor.merge.jsonInvalid")}</Alert> : null}
    </div>
  );
}

function FileProcessorParamsEditor({ draft, kind, onChange, scriptFiles }: ProcessorParamsEditorProps & { kind: FileKind; scriptFiles: ResourceOption[] }) {
	const { t } = useI18n();
	if (draft.opaque) {
		return <Alert severity="info">{t("files.processor.opaquePreserved")}</Alert>;
	}
  const params = draft.params;
  switch (draft.type) {
    case "script":
      return <ScriptProcessorParamsEditor params={params} scriptFiles={scriptFiles} onChange={onChange} />;
    case "merge":
      return <FileMergeParamsEditor kind={kind} params={params} onChange={onChange} />;
    default:
      return <KeyValueParamsEditor params={params} onChange={onChange} />;
  }
}

function draftProcessors(processors: ProcessorDetail[]): ProcessorDraft[] {
	return processors.map((processor, index) => {
		const draft = draftFromProcessor(processor, index);
		return processor.type === "script" || processor.type === "merge" ? draft : { ...draft, opaque: processor };
	});
}

function draftFromProcessor(processor: ProcessorDetail, index: number): ProcessorDraft {
  return { id: createProcessorID(index), name: stringValue(processor.name), type: processor.type || "script", params: cleanParams(processor.params ?? {}) };
}

function serializeDraft(draft: ProcessorDraft, t: Translator, kind: FileKind): ProcessorDetail {
	if (draft.opaque) return draft.opaque;
  const params = draft.type === "script"
    ? sanitizeScriptParams(draft.params)
    : draft.type === "merge"
      ? mergeParams(draft.params, kind)
      : cleanParams(draft.params);
  const name = customProcessorName(draft, (type) => processorLabel(type, t));
  return { ...(name ? { name } : {}), type: draft.type, stage: "file", ...(Object.keys(params).length ? { params } : {}) };
}

function defaultParams(type: string, kind: FileKind): Record<string, unknown> {
  switch (type) {
    case "script": return defaultScriptParams();
    case "merge": return { mode: mergeMode(kind), content: "" };
    default: return {};
  }
}

function processorLabel(type: string, t: Translator): string {
  if (type === "script" || modelProcessorLabel(type) === "script") {
    return t("model.processor.script");
  }
  if (type === "merge") return t("model.processor.merge");
  return type || t("model.processor.fallback");
}

function mergeParams(params: Record<string, unknown>, kind: FileKind): Record<string, unknown> {
  const content = typeof params.content === "string" ? params.content : "";
  const driver = requireFileDriver(kind);
  const mode = typeof params.mode === "string" && driver.processors.mergeModes.includes(params.mode as FileMergeMode)
    ? params.mode
    : mergeMode(kind);
  return { mode, ...(content ? { content } : {}) };
}

function addFileProcessorDrafts(type: string, current: ProcessorDraft[], kind: FileKind, driver: Readonly<FileDriverDefinition>): ProcessorDraft[] {
  if (type === RULE_SOURCE_REWRITE_PRESET_OPTION) {
    if (current.some(recognizeRuleSourceRewriteProcessorPreset)) return current;
    return [...current, draftFromProcessor(ruleSourceRewriteProcessorPreset(), Date.now())];
  }
  return driver.processors.adapter?.addPreset?.(type, current)
    ?? [...current, { id: createProcessorID(), name: "", type, params: defaultParams(type, kind) }];
}

function mergeMode(kind: FileKind): FileMergeMode {
  return requireFileDriver(kind).processors.mergeModes[0] ?? "yaml_overlay";
}

function createProcessorID(index = Date.now()): string {
  return createProcessorDraftId("file-processor", index);
}
