import { useEffect, useMemo, useState } from "react";
import TextField from "@mui/material/TextField";
import ToggleButton from "@mui/material/ToggleButton";
import ToggleButtonGroup from "@mui/material/ToggleButtonGroup";
import Typography from "@mui/material/Typography";

import type { FileInputValidationCode } from "~/features/files/model/input-validation";
import type { FileSourceDetail } from "~/features/files/model/types";
import { type Translator, useI18n } from "~/shared/i18n/context";
import { HighlightedTextarea } from "~/shared/ui/code-editor";

type SourceType = "inline" | "remote";

const sourceTypeOptions: SourceType[] = ["inline", "remote"];

export function FileSourceEditor({
  contentLabel,
  defaultValue,
  inlineFallback = "",
  preserveImplicit = false,
  language = "yaml",
  onValidityChange,
  placeholder,
  remoteURLPlaceholder = "https://example.com/file.yaml",
  validateSource,
}: {
  contentLabel?: string;
  defaultValue?: FileSourceDetail;
  inlineFallback?: string;
  preserveImplicit?: boolean;
  language?: string;
  onValidityChange?: (valid: boolean) => void;
  placeholder?: string;
  remoteURLPlaceholder?: string;
  validateSource?: (source: FileSourceDetail) => FileInputValidationCode | null;
}) {
  const { t } = useI18n();
  const initial = defaultValue ?? { type: "inline", content: "" };
  const [sourceType, setSourceType] = useState(initialSourceType(initial));
  const [preserveImplicitSource, setPreserveImplicitSource] = useState(preserveImplicit && isImplicitSource(initial));
  const [content, setContent] = useState(initial.content ?? (preserveImplicit && isImplicitSource(initial) ? inlineFallback : ""));
  const [url, setURL] = useState(initial?.remote?.url ?? "");
  const [userAgent, setUserAgent] = useState(initial?.remote?.user_agent ?? "");
  const [proxy, setProxy] = useState(initial?.remote?.proxy ?? "");
  const [timeoutMS, setTimeoutMS] = useState(numberInputValue(initial?.remote?.timeout_ms));
  const [cacheTTLSeconds] = useState(initial?.remote?.cache_ttl_seconds);
  const serialized = useMemo(
    () => JSON.stringify(preserveImplicitSource ? {} : serializeSource({ cacheTTLSeconds, content, proxy, sourceType, timeoutMS, url, userAgent })),
    [cacheTTLSeconds, content, preserveImplicitSource, proxy, sourceType, timeoutMS, url, userAgent],
  );
  const validationError = validateSource ? validateSource(preserveImplicitSource
    ? {}
    : sourceType === "inline"
      ? { type: "inline", content }
    : { type: "remote", remote: { url } }) : null;

  useEffect(() => onValidityChange?.(validationError === null), [onValidityChange, validationError]);

  function selectSourceType(nextType: SourceType | null) {
    if (!nextType || nextType === sourceType) return;
    setSourceType(nextType);
    setPreserveImplicitSource(false);
    if (nextType === "inline" && initial.content === undefined) {
      setContent((current) => current || inlineFallback);
    }
  }

  return (
    <div className="grid gap-4">
      <input name="source" type="hidden" value={serialized} />
      <ToggleButtonGroup
        exclusive
        aria-label={t("files.form.sourceMode")}
        color="primary"
        fullWidth
        size="small"
        value={sourceType}
        onChange={(_, value: SourceType | null) => selectSourceType(value)}
      >
        {sourceTypeOptions.map((option) => {
          const label = option === "inline" ? t("files.form.local") : t("files.form.remote");
          return <ToggleButton aria-label={label} key={option} value={option}>{label}</ToggleButton>;
        })}
      </ToggleButtonGroup>
      {sourceType === "inline" ? (
        <div className="grid gap-1.5">
          <HighlightedTextarea
            showLineNumbers
            label={contentLabel ?? t("files.form.content")}
            language={language}
            minRows={8}
            placeholder={placeholder}
            value={content}
            onChange={(event) => {
              setPreserveImplicitSource(false);
              setContent(event.target.value);
            }}
          />
          {validationError ? <Typography color="error" role="alert" variant="caption">{sourceValidationMessage(validationError, t)}</Typography> : null}
        </div>
      ) : null}
      {sourceType === "remote" ? (
        <div className="grid gap-4">
          <TextField error={Boolean(validationError)} fullWidth helperText={validationError ? sourceValidationMessage(validationError, t) : undefined} label={t("files.form.remoteUrl")} placeholder={remoteURLPlaceholder} value={url} onChange={(event) => setURL(event.target.value)} />
          <div className="grid gap-4 md:grid-cols-2">
            <TextField fullWidth label="User-Agent" value={userAgent} onChange={(event) => setUserAgent(event.target.value)} />
            <TextField fullWidth label={t("files.form.proxy")} placeholder="http://127.0.0.1:7890" value={proxy} onChange={(event) => setProxy(event.target.value)} />
            <TextField fullWidth label={t("files.form.timeoutMs")} type="number" value={timeoutMS} onChange={(event) => setTimeoutMS(event.target.value)} />
          </div>
          <Typography color="text.secondary" variant="body2">
            {t("files.form.remoteDescription")}
          </Typography>
        </div>
      ) : null}
    </div>
  );
}

function sourceValidationMessage(error: FileInputValidationCode, t: Translator): string {
  switch (error) {
    case "source_json_invalid": return t("files.form.sourceJsonInvalid");
    case "source_json_object_required": return t("files.form.sourceJsonObjectRequired");
    case "source_remote_url_invalid": return t("files.form.sourceRemoteURLInvalid");
  }
}

type SourceDraft = {
  cacheTTLSeconds?: number;
  content: string;
  proxy: string;
  sourceType: SourceType;
  timeoutMS: string;
  url: string;
  userAgent: string;
};

function initialSourceType(source?: FileSourceDetail): SourceType {
  if (source?.type === "remote" || source?.remote?.url) return "remote";
  return "inline";
}

function isImplicitSource(source: FileSourceDetail): boolean {
  return !source.type && source.content === undefined && !source.remote?.url;
}

function serializeSource(draft: SourceDraft): Record<string, unknown> {
  if (draft.sourceType === "inline") {
    return { type: "inline", content: draft.content };
  }
  return cleanSource({
    type: "remote",
    remote: cleanSource({
      url: draft.url,
      user_agent: draft.userAgent,
      proxy: draft.proxy,
      timeout_ms: numberOrUndefined(draft.timeoutMS),
      cache_ttl_seconds: draft.cacheTTLSeconds,
    }),
  });
}

function cleanSource(source: Record<string, unknown>): Record<string, unknown> {
  return Object.fromEntries(Object.entries(source).filter(([, value]) => {
    if (value === undefined || value === null || value === "") return false;
    if (typeof value === "object" && !Array.isArray(value)) return Object.keys(value).length > 0;
    return true;
  }));
}

function numberInputValue(value: unknown): string {
  return typeof value === "number" && Number.isFinite(value) ? String(value) : "";
}

function numberOrUndefined(value: string): number | undefined {
  if (!value.trim()) return undefined;
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : undefined;
}
