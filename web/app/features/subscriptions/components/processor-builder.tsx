import Checkbox from "@mui/material/Checkbox";
import FormControlLabel from "@mui/material/FormControlLabel";
import TextField from "@mui/material/TextField";

import nodeNameNormalizationScript from "~/features/subscriptions/processors/scripts/node-name-normalization.js?raw";
import { useUICapabilities } from "~/shared/capabilities/context";
import { type Translator, useI18n } from "~/shared/i18n/context";
import { DEFAULT_PROBE_URL } from "~/shared/probe/defaults";
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
  arrayValue,
  cleanParams,
  createProcessorDraftId,
  customProcessorName,
  linesToList,
  listToLines,
  listToText,
  numberInputValue,
  numberOrEmpty,
  type ProcessorDraft,
  stringValue,
  textToList,
} from "~/shared/processors/model";
import type { ProcessorDetail, ResourceOption } from "~/shared/resources/types";
import { SelectField } from "~/shared/ui/form-fields";
import { ProbeURLField } from "~/shared/ui/probe-url-field";

const fields = ["name", "type", "server"];
const fieldOptions = fields.map((field) => ({ value: field, label: field }));
const informationNodePattern =
  "(?i)(网址|官网|流量|剩余|时间|应急|套餐|订阅|公告|重置|过期|到期|bandwidth|traffic|quota|reset|expire|expiry|expiration)";
const probeRuntimeDefaults = {
  method: "url_test",
  core: "sing-box",
  url: DEFAULT_PROBE_URL,
  ntpServer: "time.apple.com",
  failMode: "keep",
  timeoutMS: 5000,
  attempts: 1,
  concurrency: 10,
  cacheTTLSeconds: 0,
};

export function ProcessorBuilder({ defaultValue = [], onDirty, scriptFiles = [] }: { defaultValue?: ProcessorDetail[]; onDirty?: () => void; scriptFiles?: ResourceOption[] }) {
  const { t } = useI18n();
  const { hasFeature } = useUICapabilities();
  const options = processorOptions(t, hasFeature("probe.enabled"));

  function ParamsEditor(props: ProcessorParamsEditorProps) {
    return <ProcessorParamsEditor {...props} scriptFiles={scriptFiles} />;
  }

  return (
    <ProcessorEditorList
      addProcessorDrafts={(type, current) => addSubscriptionProcessorDrafts(type, current, t)}
      createDraftId={createProcessorID}
      defaultParams={defaultParams}
      defaultType="filter"
      defaultValue={defaultValue}
      draftProcessors={draftProcessors}
      paramsEditor={ParamsEditor}
      processorOptions={options}
      serializeDraft={(draft) => serializeDraft(draft, t)}
      onDirty={onDirty}
    />
  );
}

function ProcessorParamsEditor({ draft, onChange, scriptFiles }: ProcessorParamsEditorProps & { scriptFiles: ResourceOption[] }) {
  const { t } = useI18n();
  const params = draft.params;
  switch (draft.type) {
    case "filter": {
      const match = stringValue(params.match) || "regex";
      return (
        <>
          <SelectField label={t("processors.filter.action")} options={filterActionOptions(t)} value={stringValue(params.action) || "keep"} onChange={(value) => onChange({ action: value })} />
          <SelectField label={t("processors.filter.field")} options={fieldOptions} value={stringValue(params.field) || "name"} onChange={(value) => onChange({ field: value })} />
          <SelectField label={t("processors.filter.match")} options={filterMatchOptions(t)} value={match} onChange={(value) => onChange({ match: value, pattern: "", values: [] })} />
          {match === "in" ? (
            <TextField fullWidth label={t("processors.filter.value")} placeholder="ss, vmess" value={listToText(params.values)} onChange={(event) => onChange({ values: textToList(event.target.value) })} />
          ) : (
            <TextField fullWidth label={t("processors.filter.pattern")} value={stringValue(params.pattern)} onChange={(event) => onChange({ pattern: event.target.value })} />
          )}
        </>
      );
    }
    case "dedup":
      return (
        <>
          <SelectField
            label={t("processors.dedup.strategy")}
            options={[
              { value: "identity", label: t("processors.dedup.identity") },
              { value: "name", label: t("labels.name") },
              { value: "fields", label: t("processors.dedup.fields") },
            ]}
            value={stringValue(params.strategy) || "identity"}
            onChange={(value) => onChange({ strategy: value, fields: value === "fields" ? arrayValue(params.fields) : [] })}
          />
          {stringValue(params.strategy) === "fields" ? (
            <TextField fullWidth label={t("processors.field")} placeholder="server, port" value={listToText(params.fields)} onChange={(event) => onChange({ fields: textToList(event.target.value) })} />
          ) : null}
        </>
      );
    case "rename": {
      const mode = stringValue(params.mode);
      return (
        <>
          <FormControlLabel
            className="md:col-span-2"
            control={<Checkbox checked={params.trim === true} onChange={(event) => onChange({ trim: event.target.checked ? true : undefined })} />}
            label={t("processors.rename.stripWhitespace")}
          />
          <TextField fullWidth multiline className="md:col-span-2" label={t("processors.rename.removeFragments")} minRows={4} placeholder={t("processors.rename.removeFragmentsPlaceholder")} value={listToLines(params.strip)} onChange={(event) => onChange({ strip: linesToList(event.target.value) })} />
          <SelectField label={t("processors.rename.mode")} options={renameModeOptions(t)} value={mode} onChange={(value) => onChange({ mode: value, pattern: "", replacement: "", value: "" })} />
          {mode === "replace" ? (
            <>
              <TextField fullWidth label={t("processors.rename.pattern")} value={stringValue(params.pattern)} onChange={(event) => onChange({ pattern: event.target.value })} />
              <TextField fullWidth label={t("processors.rename.replacement")} value={stringValue(params.replacement)} onChange={(event) => onChange({ replacement: event.target.value })} />
            </>
          ) : null}
          {mode === "prefix" || mode === "suffix" || mode === "template" ? (
            <TextField fullWidth label={t("processors.rename.content")} value={stringValue(params.value)} onChange={(event) => onChange({ value: event.target.value })} />
          ) : null}
        </>
      );
    }
    case "sort":
      return <TextField fullWidth label={t("processors.sort.field")} placeholder="+name, -server" value={stringValue(params.by) || "+name"} onChange={(event) => onChange({ by: event.target.value })} />;
    case "quick_settings":
      return (
        <>
          <SelectField label="UDP" options={quickValues(t)} value={stringValue(params.udp)} onChange={(value) => onChange({ udp: value })} />
          <SelectField label="TFO" options={quickValues(t)} value={stringValue(params.tfo)} onChange={(value) => onChange({ tfo: value })} />
		  <SelectField label={t("processors.quick.reuse")} options={quickValues(t)} value={stringValue(params.reuse)} onChange={(value) => onChange({ reuse: value })} />
          <SelectField label={t("processors.quick.allowInsecure")} options={quickValues(t)} value={stringValue(params.allow_insecure)} onChange={(value) => onChange({ allow_insecure: value })} />
          <SelectField label="VMess AEAD" options={quickValues(t)} value={stringValue(params.vmess_aead)} onChange={(value) => onChange({ vmess_aead: value })} />
        </>
      );
    case "probe": {
      const method = stringValue(params.method) || probeRuntimeDefaults.method;
      const showNTPServer = method === "udp_ntp";
      const showURLTarget = method === "url_test";
      return (
        <>
          <SelectField label={t("processors.probe.method")} options={probeMethodOptions()} value={method} onChange={(value) => onChange(probeMethodPatch(value))} />
          {showNTPServer ? (
            <TextField fullWidth label={t("processors.probe.ntpServer")} value={stringValue(params.ntp_server)} onChange={(event) => onChange({ ntp_server: event.target.value })} />
          ) : null}
          {showURLTarget ? (
            <>
              <ProbeURLField label={t("processors.probe.url")} value={stringValue(params.url)} onChange={(url) => onChange({ url })} />
              <TextField fullWidth label={t("processors.probe.expectedStatus")} placeholder="200-299" value={stringValue(params.expected_status)} onChange={(event) => onChange({ expected_status: event.target.value })} />
            </>
          ) : null}
          <TextField fullWidth label={t("files.form.timeoutMs")} type="number" value={numberInputValue(params.timeout_ms)} onChange={(event) => onChange({ timeout_ms: numberOrEmpty(event.target.value) })} />
          <TextField fullWidth label={t("processors.probe.attempts")} type="number" value={numberInputValue(params.attempts)} onChange={(event) => onChange({ attempts: numberOrEmpty(event.target.value) })} />
          <TextField fullWidth label={t("processors.probe.concurrency")} type="number" value={numberInputValue(params.concurrency)} onChange={(event) => onChange({ concurrency: numberOrEmpty(event.target.value) })} />
          <TextField fullWidth label={t("processors.probe.cacheTTLSeconds")} type="number" value={numberInputValue(params.cache_ttl_seconds)} onChange={(event) => onChange({ cache_ttl_seconds: numberOrEmpty(event.target.value) })} />
          <SelectField label={t("processors.sort")} options={probeSortOptions(t)} value={stringValue(params.sort)} onChange={(value) => onChange({ sort: value })} />
          <SelectField
            label={t("processors.failMode")}
            options={[
              { value: "keep", label: t("processors.filter.keep") },
              { value: "drop", label: t("processors.failMode.drop") },
              { value: "error", label: t("processors.failMode.error") },
            ]}
            value={stringValue(params.fail_mode) || "keep"}
            onChange={(value) => onChange({ fail_mode: value })}
          />
          <FormControlLabel
            className="md:col-span-2"
            control={<Checkbox checked={params.annotate === true} onChange={(event) => onChange({ annotate: event.target.checked ? true : undefined })} />}
            label={t("processors.probe.annotate")}
          />
        </>
      );
    }
    case "script":
      return <ScriptProcessorParamsEditor params={params} scriptFiles={scriptFiles} onChange={onChange} />;
    default:
      return <KeyValueParamsEditor params={params} onChange={onChange} />;
  }
}

function processorLabel(type: string, t: Translator): string {
  return processorOptions(t).find((option) => option.value === type)?.label ?? type;
}

function draftProcessors(processors: ProcessorDetail[]): ProcessorDraft[] {
  return processors.map(draftFromProcessor);
}

function draftFromProcessor(processor: ProcessorDetail, index: number): ProcessorDraft {
  const type = processor.type || "filter";
  const params = type === "probe" ? { ...defaultParams(type), ...(processor.params ?? {}) } : processor.params ?? {};
  return {
    id: createProcessorID(index),
    name: stringValue(processor.name),
    type,
    params: cleanParams(params),
  };
}

function serializeDraft(draft: ProcessorDraft, t: Translator): ProcessorDetail {
  const params = draft.type === "script" ? sanitizeScriptParams(draft.params) : draft.type === "probe" ? sanitizeProbeParams(draft.params) : cleanParams(draft.params);
  const name = customProcessorName(draft, (type) => processorLabel(type, t));
  return {
    ...(name ? { name } : {}),
    type: draft.type,
    stage: defaultStage(),
    ...(Object.keys(params).length ? { params } : {}),
  };
}

function sanitizeProbeParams(params: Record<string, unknown>): Record<string, unknown> {
  const out = cleanParams(params);
  const method = stringValue(out.method) || probeRuntimeDefaults.method;
  out.method = method;
  delete out.layer;
  if (method === "tcp_connect") {
    delete out.core;
  } else {
    out.core = probeRuntimeDefaults.core;
  }
  if (method !== "udp_ntp") {
    delete out.ntp_server;
  }
  if (method !== "url_test") {
    delete out.url;
    delete out.expected_status;
  }
  return out;
}

function probeMethodPatch(method: string): Record<string, unknown> {
  switch (method) {
    case "tcp_connect":
      return { method, core: undefined, url: undefined, expected_status: undefined, ntp_server: undefined };
    case "udp_ntp":
      return { method, core: probeRuntimeDefaults.core, url: undefined, expected_status: undefined, ntp_server: probeRuntimeDefaults.ntpServer };
    default:
      return { method: "url_test", core: probeRuntimeDefaults.core, url: probeRuntimeDefaults.url, expected_status: undefined, ntp_server: undefined };
  }
}

function processorOptions(t: Translator, probeEnabled = true) {
  return [
    { value: "filter", label: t("processors.filter") },
    { value: "information_filter_preset", label: t("processors.filter.infoPresetOption") },
    { value: "dedup", label: t("processors.dedup") },
    { value: "rename", label: t("processors.nameOperation") },
    { value: "node_name_normalization_preset", label: t("processors.rename.normalizationPresetOption") },
    { value: "sort", label: t("processors.sort") },
    { value: "quick_settings", label: t("processors.quickSettings") },
    ...(probeEnabled ? [{ value: "probe", label: t("processors.probe") }] : []),
    { value: "script", label: t("model.processor.script") },
  ];
}

function addSubscriptionProcessorDrafts(type: string, current: ProcessorDraft[], t: Translator): ProcessorDraft[] {
  if (type === "node_name_normalization_preset") {
    return [...current, {
      id: createProcessorID(),
      name: t("processors.rename.normalizationPresetName"),
      type: "script",
      params: {
        source: {
          type: "inline",
          content: nodeNameNormalizationScript,
        },
      },
    }];
  }
  if (type !== "information_filter_preset") {
    return [...current, {
      id: createProcessorID(),
      name: "",
      type,
      params: defaultParams(type),
    }];
  }
  const preset: ProcessorDraft = {
    id: createProcessorID(),
    name: t("processors.filter.infoPresetName"),
    type: "filter",
    params: {
      action: "drop",
      field: "name",
      match: "regex",
      pattern: informationNodePattern,
    },
  };
  return [...current, preset];
}

function filterActionOptions(t: Translator) {
  return [
    { value: "keep", label: t("processors.filter.keep") },
    { value: "drop", label: t("processors.filter.drop") },
  ];
}

function filterMatchOptions(t: Translator) {
  return [
    { value: "regex", label: t("processors.filter.regex") },
    { value: "in", label: t("processors.filter.in") },
  ];
}

function renameModeOptions(t: Translator) {
  return [
    { value: "", label: t("processors.rename.none") },
    { value: "replace", label: t("processors.rename.replace") },
    { value: "prefix", label: t("processors.rename.prefix") },
    { value: "suffix", label: t("processors.rename.suffix") },
    { value: "template", label: t("processors.rename.template") },
  ];
}

function quickValues(t: Translator) {
  return [
    { value: "", label: t("processors.quick.default") },
    { value: "enabled", label: t("processors.quick.enabled") },
    { value: "disabled", label: t("processors.quick.disabled") },
  ];
}

function probeMethodOptions() {
  return [
    { value: "tcp_connect", label: "tcp_connect" },
    { value: "udp_ntp", label: "udp_ntp" },
    { value: "url_test", label: "url_test" },
  ];
}

function probeSortOptions(t: Translator) {
  return [
    { value: "", label: t("processors.probe.sortDefault") },
    { value: "duration", label: t("processors.probe.sortDuration") },
  ];
}

function defaultStage(): string {
  return "nodes";
}

function defaultParams(type: string): Record<string, unknown> {
  switch (type) {
    case "filter":
      return { action: "keep", field: "name", match: "regex", pattern: "" };
    case "dedup":
      return { strategy: "identity" };
    case "rename":
      return { trim: true };
    case "sort":
      return { by: "+name" };
    case "quick_settings":
      return {};
    case "probe":
      return {
        method: probeRuntimeDefaults.method,
        core: probeRuntimeDefaults.core,
        url: probeRuntimeDefaults.url,
        timeout_ms: probeRuntimeDefaults.timeoutMS,
        attempts: probeRuntimeDefaults.attempts,
        concurrency: probeRuntimeDefaults.concurrency,
        cache_ttl_seconds: probeRuntimeDefaults.cacheTTLSeconds,
        fail_mode: probeRuntimeDefaults.failMode,
      };
    case "script":
      return defaultScriptParams();
    default:
      return {};
  }
}

function createProcessorID(index = Date.now()): string {
  return createProcessorDraftId("processor", index);
}
