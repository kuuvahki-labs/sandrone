import { useId, useMemo, useState } from "react";
import FormControl from "@mui/material/FormControl";
import InputLabel from "@mui/material/InputLabel";
import Select from "@mui/material/Select";
import TextField from "@mui/material/TextField";
import ToggleButton from "@mui/material/ToggleButton";
import ToggleButtonGroup from "@mui/material/ToggleButtonGroup";

import {
  millisecondsToSecondsInput,
  secondsInputToMilliseconds,
} from "~/shared/api/duration";
import { useI18n } from "~/shared/i18n/context";
import {
  cleanParams,
  keyValueTextToObject,
  numberOrEmpty,
  objectToKeyValueText,
  stringValue,
} from "~/shared/processors/model";
import { resourceOptionText } from "~/shared/resources/labels";
import type { RemoteInputDefaults, ResourceOption } from "~/shared/resources/types";
import { HighlightedTextarea } from "~/shared/ui/code-editor";

type ScriptSourceMode = "inline" | "file" | "remote";

const scriptParamFields = new Set(["source", "engine", "args", "timeout_ms", "id", "permissions"]);

export function ScriptProcessorParamsEditor({
  defaultTimeoutMS,
  onChange,
  params,
  remoteDefaults,
  scriptFiles = [],
}: {
  defaultTimeoutMS?: number;
  onChange: (patch: Record<string, unknown>) => void;
  params: Record<string, unknown>;
  remoteDefaults: RemoteInputDefaults;
  scriptFiles?: ResourceOption[];
}) {
  const { t } = useI18n();
  const source = scriptSourceFromParams(params);
  const [mode, setMode] = useState<ScriptSourceMode>(() => source.type);
  const [argsText, setArgsText] = useState(() => objectToKeyValueText(params.args));
  const currentFileName = source.type === "file" ? source.name : stringValue(params.path);
  const scriptFileOptions = useMemo(() => scriptFileOptionsFor(scriptFiles, currentFileName), [currentFileName, scriptFiles]);

  function selectMode(nextMode: ScriptSourceMode | null) {
    if (!nextMode || nextMode === mode) return;
    setMode(nextMode);
    if (nextMode === "inline") {
      onChange(sourcePatch({ type: "inline", content: source.type === "inline" ? source.content : defaultScriptContent() }));
      return;
    }
    if (nextMode === "file") {
      onChange(sourcePatch({ type: "file", name: currentFileName || scriptFileOptions[0]?.value || "" }));
      return;
    }
    onChange(sourcePatch({ type: "remote", remote: source.type === "remote" ? source.remote : {}, sha256: source.type === "remote" ? source.sha256 : "" }));
  }

  function updateArgs(value: string) {
    setArgsText(value);
    onChange({ args: keyValueTextToObject(value) });
  }

  return (
    <>
      <ToggleButtonGroup
        exclusive
        aria-label={t("script.source")}
        className="md:col-span-2"
        color="primary"
        fullWidth
        size="small"
        value={mode}
        onChange={(_, value: ScriptSourceMode | null) => selectMode(value)}
      >
        <ToggleButton aria-label={t("script.inline")} value="inline">{t("script.inline")}</ToggleButton>
        <ToggleButton aria-label={t("script.fileMode")} value="file">{t("script.fileMode")}</ToggleButton>
        <ToggleButton aria-label={t("script.remote")} value="remote">{t("script.remote")}</ToggleButton>
      </ToggleButtonGroup>
      {mode === "file" ? (
        <ScriptFileSelect
          options={scriptFileOptions}
          value={source.type === "file" ? source.name : ""}
          t={t}
          onChange={(value) => onChange(sourcePatch({ type: "file", name: value }))}
        />
      ) : null}
      {mode === "inline" ? (
        <HighlightedTextarea showLineNumbers className="md:col-span-2" label={t("script.code")} language="javascript" minRows={8} value={source.type === "inline" ? source.content : ""} onChange={(event) => onChange(sourcePatch({ type: "inline", content: event.target.value }))} />
      ) : null}
      {mode === "remote" ? (
        <RemoteScriptFields
          source={source.type === "remote" ? source : { type: "remote", remote: {}, sha256: "" }}
          remoteDefaults={remoteDefaults}
          t={t}
          onChange={(nextSource) => onChange(sourcePatch(nextSource))}
        />
      ) : null}
      <HighlightedTextarea className="md:col-span-2" label={t("script.args")} language="text" minRows={4} placeholder={"flag=true\nthreshold=2"} value={argsText} onChange={(event) => updateArgs(event.target.value)} />
      <TextField
        fullWidth
        helperText={t("script.executionTimeoutHint")}
        label={t("script.executionTimeoutMs")}
        placeholder={millisecondsToSecondsInput(defaultTimeoutMS)}
        slotProps={{ htmlInput: durationInputProps }}
        type="number"
        value={millisecondsToSecondsInput(params.timeout_ms)}
        onChange={(event) => onChange({ timeout_ms: secondsInputToMilliseconds(event.target.value) ?? "" })}
      />
    </>
  );
}

export function defaultScriptParams(): Record<string, unknown> {
  return { source: { type: "inline", content: defaultScriptContent() } };
}

export function sanitizeScriptParams(params: Record<string, unknown>): Record<string, unknown> {
  const cleaned = cleanParams(params);
  const sanitized = Object.fromEntries(Object.entries(cleaned).filter(([key]) => scriptParamFields.has(key)));
  if (!(typeof sanitized.timeout_ms === "number" && sanitized.timeout_ms > 0)) {
    delete sanitized.timeout_ms;
  }
  const source = sanitizeScriptSource(scriptSourceFromParams(params));
  if (source) {
    sanitized.source = source;
  } else {
    delete sanitized.source;
  }
  return sanitized;
}

function ScriptFileSelect({ onChange, options, t, value }: { onChange: (value: string) => void; options: ScriptFileOption[]; t: ReturnType<typeof useI18n>["t"]; value: string }) {
  const labelId = useId();
  const selectId = useId();
  return (
    <FormControl className="md:col-span-2" fullWidth>
      <InputLabel htmlFor={selectId} id={labelId}>{t("script.file")}</InputLabel>
      <Select
        native
        id={selectId}
        label={t("script.file")}
        labelId={labelId}
        value={value}
        onChange={(event) => onChange(String(event.target.value))}
      >
        <option disabled value="">{t("script.selectFile")}</option>
        {options.map((option) => (
          <option key={option.value} value={option.value}>{option.label}</option>
        ))}
      </Select>
    </FormControl>
  );
}

function RemoteScriptFields({ onChange, remoteDefaults, source, t }: { onChange: (source: RemoteScriptSourceDraft) => void; remoteDefaults: RemoteInputDefaults; source: RemoteScriptSourceDraft; t: ReturnType<typeof useI18n>["t"] }) {
  const remote = source.remote ?? {};
  function updateRemote(patch: Partial<RemoteScriptDraft>) {
    onChange({ type: "remote", remote: cleanRemoteScriptDraft({ ...remote, ...patch }), sha256: source.sha256 });
  }
  return (
    <>
      <TextField className="md:col-span-2" fullWidth label={t("script.remoteUrl")} value={remote.url ?? ""} onChange={(event) => updateRemote({ url: event.target.value })} />
      <TextField fullWidth label={t("script.remoteUserAgent")} placeholder={remoteDefaults.userAgent} value={remote.user_agent ?? ""} onChange={(event) => updateRemote({ user_agent: event.target.value })} />
      <TextField fullWidth label={t("script.remoteProxy")} placeholder={remoteDefaults.proxy || "http://127.0.0.1:7890"} value={remote.proxy ?? ""} onChange={(event) => updateRemote({ proxy: event.target.value })} />
      <TextField fullWidth label={t("script.remoteTimeoutMs")} placeholder={millisecondsToSecondsInput(remoteDefaults.timeoutMS)} slotProps={{ htmlInput: durationInputProps }} type="number" value={millisecondsToSecondsInput(remote.timeout_ms)} onChange={(event) => updateRemote({ timeout_ms: secondsInputToMilliseconds(event.target.value) ?? "" })} />
      <TextField fullWidth label={t("cache.remoteFetchTTLSeconds")} placeholder={String(remoteDefaults.cacheTTLSeconds)} type="number" value={positiveNumberInput(remote.cache_ttl_seconds)} onChange={(event) => updateRemote({ cache_ttl_seconds: numberOrEmpty(event.target.value) })} />
      <TextField fullWidth label="SHA-256" value={source.sha256 ?? ""} onChange={(event) => onChange({ type: "remote", remote, sha256: event.target.value.trim() })} />
    </>
  );
}

type ScriptFileOption = {
  label: string;
  value: string;
};

type RemoteScriptDraft = {
  url?: string;
  user_agent?: string;
  proxy?: string;
  timeout_ms?: number | "";
  cache_ttl_seconds?: number | "";
};

type ScriptSourceDraft =
  | { type: "inline"; content: string }
  | { type: "file"; name: string }
  | RemoteScriptSourceDraft;

type RemoteScriptSourceDraft = { type: "remote"; remote: RemoteScriptDraft; sha256?: string };

function scriptFileOptionsFor(files: ResourceOption[], currentPath: string): ScriptFileOption[] {
  const options = files
    .filter((file) => file.name.toLowerCase().endsWith(".js"))
    .map((file) => ({ label: resourceOptionText(file), value: file.name }))
    .sort((left, right) => left.value.localeCompare(right.value));
  const seen = new Set(options.map((option) => option.value));
  const current = currentPath.trim();
  if (current && !seen.has(current)) {
    options.unshift({ label: current, value: current });
  }
  return options;
}

function sourcePatch(source: ScriptSourceDraft): Record<string, unknown> {
  return { source: sanitizeScriptSource(source) ?? source, path: "", content: "" };
}

function scriptSourceFromParams(params: Record<string, unknown>): ScriptSourceDraft {
  const source = objectValue(params.source);
  const type = stringValue(source.type);
  if (type === "inline") {
    return { type: "inline", content: stringValue(source.content) };
  }
  if (type === "file") {
    return { type: "file", name: stringValue(source.name) };
  }
  if (type === "remote") {
    return { type: "remote", remote: remoteScriptDraftFrom(source.remote), sha256: stringValue(source.sha256) };
  }
  if (stringValue(params.content)) {
    return { type: "inline", content: stringValue(params.content) };
  }
  if (stringValue(params.path)) {
    return { type: "file", name: stringValue(params.path) };
  }
  return { type: "inline", content: defaultScriptContent() };
}

function sanitizeScriptSource(source: ScriptSourceDraft): Record<string, unknown> | null {
  switch (source.type) {
    case "inline":
      return stringValue(source.content) ? { type: "inline", content: source.content } : null;
    case "file":
      return stringValue(source.name) ? { type: "file", name: source.name } : null;
    case "remote": {
      const remote = cleanRemoteScriptDraft(source.remote);
      if (!stringValue(remote.url)) return null;
      const out: Record<string, unknown> = { type: "remote", ...(Object.keys(remote).length ? { remote } : {}) };
      if (stringValue(source.sha256)) {
        out.sha256 = source.sha256;
      }
      return out;
    }
  }
}

function remoteScriptDraftFrom(value: unknown): RemoteScriptDraft {
  const remote = objectValue(value);
  return cleanRemoteScriptDraft({
    url: stringValue(remote.url),
    user_agent: stringValue(remote.user_agent),
    proxy: stringValue(remote.proxy),
    timeout_ms: numberOrEmpty(String(remote.timeout_ms ?? "")),
    cache_ttl_seconds: numberOrEmpty(String(remote.cache_ttl_seconds ?? "")),
  });
}

function cleanRemoteScriptDraft(remote: RemoteScriptDraft): RemoteScriptDraft {
  const cleaned = cleanParams(remote as Record<string, unknown>) as RemoteScriptDraft;
  if (!(typeof cleaned.timeout_ms === "number" && cleaned.timeout_ms > 0)) {
    delete cleaned.timeout_ms;
  }
  if (!(typeof cleaned.cache_ttl_seconds === "number" && cleaned.cache_ttl_seconds > 0)) {
    delete cleaned.cache_ttl_seconds;
  }
  return cleaned;
}

function positiveNumberInput(value: number | "" | undefined): string | number {
  return typeof value === "number" && value > 0 ? value : "";
}

const durationInputProps = { min: 0, step: "any" } as const;

function objectValue(value: unknown): Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value) ? value as Record<string, unknown> : {};
}

function defaultScriptContent(): string {
  return `// Guide: https://github.com/kuuvahki-labs/sandrone/blob/main/docs/how-to/write-processor-script.md
// input is the current object: node processors receive node lists, and file processors receive file content.
// Return the modified input; api exposes controlled helper capabilities.
function main(input, api) {
  return input;
}
`;
}
