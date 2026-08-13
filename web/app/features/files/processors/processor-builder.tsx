import { useCallback, useState } from "react";
import Alert from "@mui/material/Alert";

import type { FileDriverDefinition, FileMergeMode } from "~/features/files/drivers/core/file-driver";
import {
  type FileProcessorPresetCategory,
  type FileProcessorPresetPlan,
  filterForeignManagedProcessors,
  planFileProcessorPresetAddition,
} from "~/features/files/drivers/core/processor-presets";
import {
  FILE_DRIVER_REGISTRY,
  requireFileDriver,
} from "~/features/files/drivers/registry";
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
import type { SelectOption } from "~/shared/ui/form-fields";

import { FileMergeParamsEditor } from "./merge-params-editor";

const FILE_PRESET_OPTION_PREFIX = "file-preset:";
const FILE_PRESET_CATEGORIES: readonly FileProcessorPresetCategory[] = [
  "privacy",
  "network",
  "platform",
  "tailscale",
];
const FILE_PRESET_CATEGORY_LABEL_KEYS: Record<FileProcessorPresetCategory, Parameters<Translator>[0]> = {
  privacy: "processors.filePreset.category.privacy",
  network: "processors.filePreset.category.network",
  platform: "processors.filePreset.category.platform",
  tailscale: "processors.filePreset.category.tailscale",
};
const FILE_PROCESSOR_PRESET_CATALOGS = FILE_DRIVER_REGISTRY.drivers
  .map((driver) => driver.processors.presets);

type PresetNotice = {
  description: string;
  risk?: string;
  dependencyLabels: string[];
  removedLabels: string[];
};

export function FileProcessorBuilder({ defaultValue = [], kind, onDirty, onValidityChange, scriptFiles = [] }: { defaultValue?: ProcessorDetail[]; kind: FileKind; onDirty?: () => void; onValidityChange?: (valid: boolean) => void; scriptFiles?: ResourceOption[] }) {
  const { t } = useI18n();
  const [presetNotice, setPresetNotice] = useState<PresetNotice | null>(null);
  const [validationIssueCount, setValidationIssueCount] = useState(0);
  const driver = requireFileDriver(kind);
  const processorOptions: SelectOption[] = [
    { value: "script", label: t("model.processor.script") },
    ...(driver.processors.mergeModes.length ? [{ value: "merge", label: t("model.processor.merge") }] : []),
    ...FILE_PRESET_CATEGORIES.flatMap((category) => {
      const group = t(FILE_PRESET_CATEGORY_LABEL_KEYS[category]);
      return [
        ...driver.processors.presets
          .filter((preset) => preset.category === category)
          .map((preset) => ({
            value: `${FILE_PRESET_OPTION_PREFIX}${preset.id}`,
            label: t(preset.labelKey),
            group,
          })),
      ];
    }),
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
  const addProcessorDrafts = useCallback((type: string, current: ProcessorDraft[]): ProcessorDraft[] => {
    setPresetNotice(null);
    if (type.startsWith(FILE_PRESET_OPTION_PREFIX)) {
      const presetID = type.slice(FILE_PRESET_OPTION_PREFIX.length);
      const requested = driver.processors.presets.find((preset) => preset.id === presetID);
      if (!requested) throw new Error(`unknown file processor preset: ${presetID}`);
      const plan = planFileProcessorPresetAddition(
        driver.processors.presets,
        presetID,
        current.map(serializeProcessorDraft),
        t,
      );
      const addedIDs = new Set(plan.addedPresetIDs);
      setPresetNotice({
        description: t(requested.descriptionKey),
        ...(requested.riskKey ? { risk: t(requested.riskKey) } : {}),
        dependencyLabels: plan.dependencyPresetIDs
          .filter((id) => addedIDs.has(id))
          .map((id) => presetLabel(driver, id, t)),
        removedLabels: plan.removedPresetIDs.map((id) => presetLabel(driver, id, t)),
      });
      return applyFileProcessorPresetPlan(current, plan);
    }
    return [...current, { id: createProcessorID(), name: "", type, params: defaultParams(type, kind) }];
  }, [driver, kind, serializeProcessorDraft, t]);
  const normalizedDefaultValue = filterForeignManagedProcessors(
    driver.processors.presets,
    FILE_PROCESSOR_PRESET_CATALOGS,
    defaultValue,
  );

  return (
    <div className="grid gap-3">
      <ProcessorEditorList
        addProcessorDrafts={addProcessorDrafts}
        createDraftId={createProcessorID}
        defaultParams={(type) => defaultParams(type, kind)}
        defaultType="script"
        defaultValue={normalizedDefaultValue}
        draftProcessors={draftProcessors}
        paramsEditor={ParamsEditor}
        processorOptions={processorOptions}
        serializeDraft={serializeProcessorDraft}
        onDirty={onDirty}
        onValueChange={handleValueChange}
      />
      {presetNotice ? (
        <Alert severity={presetNotice.risk ? "warning" : "info"}>
          <div className="grid gap-1">
            <div><strong>{t("processors.filePreset.notice.description")}:</strong> {presetNotice.description}</div>
            {presetNotice.risk ? <div>{presetNotice.risk}</div> : null}
            {presetNotice.dependencyLabels.length ? (
              <div><strong>{t("processors.filePreset.notice.addedDependencies")}:</strong> {presetNotice.dependencyLabels.join(", ")}</div>
            ) : null}
            {presetNotice.removedLabels.length ? (
              <div><strong>{t("processors.filePreset.notice.removedConflicts")}:</strong> {presetNotice.removedLabels.join(", ")}</div>
            ) : null}
          </div>
        </Alert>
      ) : null}
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

function draftFromProcessor(processor: ProcessorDetail, index = Date.now()): ProcessorDraft {
  return { id: createProcessorID(index), name: stringValue(processor.name), type: processor.type || "script", params: cleanParams(processor.params ?? {}) };
}

function applyFileProcessorPresetPlan(
  current: readonly ProcessorDraft[],
  plan: FileProcessorPresetPlan,
): ProcessorDraft[] {
  const removals = new Set(plan.removeIndices);
  const additionsByBeforeIndex = new Map<number | null, ProcessorDraft[]>();
  for (const addition of plan.additions) {
    const drafts = additionsByBeforeIndex.get(addition.beforeIndex) ?? [];
    drafts.push(draftFromProcessor(addition.processor));
    additionsByBeforeIndex.set(addition.beforeIndex, drafts);
  }

  const next: ProcessorDraft[] = [];
  current.forEach((draft, index) => {
    next.push(...(additionsByBeforeIndex.get(index) ?? []));
    if (!removals.has(index)) next.push(draft);
  });
  next.push(...(additionsByBeforeIndex.get(null) ?? []));
  return next;
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

function mergeMode(kind: FileKind): FileMergeMode {
  return requireFileDriver(kind).processors.mergeModes[0] ?? "yaml_overlay";
}

function createProcessorID(index = Date.now()): string {
  return createProcessorDraftId("file-processor", index);
}

function presetLabel(driver: Readonly<FileDriverDefinition>, id: string, t: Translator): string {
  const preset = driver.processors.presets.find((candidate) => candidate.id === id);
  if (!preset) throw new Error(`unknown file processor preset: ${id}`);
  return t(preset.labelKey);
}
