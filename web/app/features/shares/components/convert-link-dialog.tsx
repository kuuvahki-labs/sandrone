import { useEffect, useMemo, useRef, useState } from "react";
import Alert from "@mui/material/Alert";
import Button from "@mui/material/Button";
import DialogActions from "@mui/material/DialogActions";
import FormControl from "@mui/material/FormControl";
import FormHelperText from "@mui/material/FormHelperText";
import InputLabel from "@mui/material/InputLabel";
import NativeSelect from "@mui/material/NativeSelect";
import Stack from "@mui/material/Stack";
import TextField from "@mui/material/TextField";
import ToggleButton from "@mui/material/ToggleButton";
import ToggleButtonGroup from "@mui/material/ToggleButtonGroup";
import Typography from "@mui/material/Typography";

import type { CopyShareResult } from "~/features/shares/data/create-share-actions";
import {
  buildConvertLink,
  type ConvertLinkResponse,
  type ConvertLinkSource,
  type ConvertLinkValidationError,
  maxPublicConvertContentBytes,
  validateConvertLinkInput,
} from "~/features/shares/model/convert-link";
import type { ApiClient, FormatCapabilityDirection, FormatCapabilitySummary } from "~/shared/api/client";
import { type TranslationKey, useI18n } from "~/shared/i18n/context";
import { AppDialog } from "~/shared/ui/dialogs";

import { hasSelectionWithin, selectContents } from "./share-url-selection";

interface ConvertLinkDialogProps {
  client: ApiClient;
  publicBaseUrl: string;
  onClose: () => void;
  onCopyUrl: (url: string) => Promise<CopyShareResult>;
}

export function ConvertLinkDialog({ client, publicBaseUrl, onClose, onCopyUrl }: ConvertLinkDialogProps) {
  const { t } = useI18n();
  const [capabilities, setCapabilities] = useState<FormatCapabilitySummary[]>([]);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [reloadKey, setReloadKey] = useState(0);
  const [sourceKind, setSourceKind] = useState<ConvertLinkSource["kind"]>("url");
  const [remoteURL, setRemoteURL] = useState("");
  const [content, setContent] = useState("");
  const [remoteFromFormat, setRemoteFromFormat] = useState("");
  const [contentFromFormat, setContentFromFormat] = useState("uri-list");
  const [toFormat, setToFormat] = useState("");
  const [response, setResponse] = useState<ConvertLinkResponse>("raw");
  const [copyAttempted, setCopyAttempted] = useState(false);
  const generatedURL = useRef<HTMLElement | null>(null);

  useEffect(() => {
    let active = true;
    setLoading(true);
    setLoadError(null);
    void client.listFormatCapabilities().then((result) => {
      if (!active) return;
      setCapabilities(result.items);
      setLoading(false);
    }).catch((error: unknown) => {
      if (!active) return;
      setCapabilities([]);
      setLoadError(error instanceof Error ? error.message : "");
      setLoading(false);
    });
    return () => {
      active = false;
    };
  }, [client, reloadKey]);

  const parseFormats = useMemo(() => formatNames(capabilities, "parse"), [capabilities]);
  const renderFormats = useMemo(() => formatNames(capabilities, "render"), [capabilities]);
  const selectedToFormat = renderFormats.includes(toFormat)
    ? toFormat
    : renderFormats.includes("base64")
      ? "base64"
      : (renderFormats[0] ?? "");
  const source: ConvertLinkSource = sourceKind === "url"
    ? { kind: "url", value: remoteURL }
    : { kind: "content", value: content };
  const fromFormat = sourceKind === "url" ? remoteFromFormat : contentFromFormat;
  const linkInput = { fromFormat, publicBaseUrl, response, source, toFormat: selectedToFormat };
  const validationError = validateConvertLinkInput(linkInput);
  const publicURL = validationError ? "" : buildConvertLink(linkInput);
  const showValidation = copyAttempted || source.value.length > 0;

  async function copyLink() {
    setCopyAttempted(true);
    if (!publicURL) return;
    if (!(await onCopyUrl(publicURL)).copied) {
      selectContents(generatedURL.current);
    }
  }

  return (
    <AppDialog title={t("shares.convert.title")} onClose={onClose}>
      <Stack spacing={2}>
        <ToggleButtonGroup
          exclusive
          fullWidth
          aria-label={t("shares.convert.sourceType")}
          value={sourceKind}
          onChange={(_event, value: ConvertLinkSource["kind"] | null) => {
            if (value) {
              setSourceKind(value);
              setCopyAttempted(false);
            }
          }}
        >
          <ToggleButton value="url">{t("shares.convert.source.url")}</ToggleButton>
          <ToggleButton value="content">{t("shares.convert.source.content")}</ToggleButton>
        </ToggleButtonGroup>

        {sourceKind === "url" ? (
          <TextField
            fullWidth
            error={showValidation && (validationError === "url_invalid" || validationError === "url_scheme" || validationError === "source_required")}
            helperText={showValidation ? validationMessage(validationError, t) : t("shares.convert.urlHelper")}
            label={t("shares.convert.url")}
            type="url"
            value={remoteURL}
            onChange={(event) => setRemoteURL(event.target.value)}
          />
        ) : (
          <TextField
            fullWidth
            multiline
            error={showValidation && (validationError === "content_too_large" || validationError === "source_required")}
            helperText={showValidation ? validationMessage(validationError, t) : t("shares.convert.contentHelper", { bytes: maxPublicConvertContentBytes })}
            label={t("shares.convert.content")}
            minRows={4}
            value={content}
            onChange={(event) => setContent(event.target.value)}
          />
        )}

        <FormControl disabled={loading || loadError !== null} fullWidth>
          <InputLabel htmlFor="convert-from-format" shrink>{t("shares.convert.fromFormat")}</InputLabel>
          <NativeSelect
            id="convert-from-format"
            inputProps={{ "aria-label": t("shares.convert.fromFormat") }}
            value={fromFormat}
            onChange={(event) => {
              if (sourceKind === "url") setRemoteFromFormat(event.target.value);
              else setContentFromFormat(event.target.value);
            }}
          >
            {sourceKind === "url" ? <option value="">{t("shares.convert.formatAuto")}</option> : null}
            {parseFormats.map((format) => <option key={format} value={format}>{format}</option>)}
          </NativeSelect>
        </FormControl>

        <FormControl disabled={loading || loadError !== null} error={copyAttempted && validationError === "to_format_required"} fullWidth>
          <InputLabel htmlFor="convert-to-format" shrink>{t("shares.convert.toFormat")}</InputLabel>
          <NativeSelect
            id="convert-to-format"
            inputProps={{ "aria-label": t("shares.convert.toFormat") }}
            value={selectedToFormat}
            onChange={(event) => setToFormat(event.target.value)}
          >
            {renderFormats.map((format) => <option key={format} value={format}>{format}</option>)}
          </NativeSelect>
          {copyAttempted && validationError === "to_format_required" ? <FormHelperText>{validationMessage(validationError, t)}</FormHelperText> : null}
        </FormControl>

        <FormControl fullWidth>
          <InputLabel htmlFor="convert-response" shrink>{t("shares.convert.response")}</InputLabel>
          <NativeSelect
            id="convert-response"
            inputProps={{ "aria-label": t("shares.convert.response") }}
            value={response}
            onChange={(event) => setResponse(event.target.value as ConvertLinkResponse)}
          >
            <option value="raw">{t("shares.convert.response.raw")}</option>
            <option value="json">{t("shares.convert.response.json")}</option>
          </NativeSelect>
        </FormControl>

        {loading ? <Typography color="text.secondary">{t("shares.convert.loading")}</Typography> : null}
        {loadError !== null ? (
          <Alert
            action={<Button color="inherit" size="small" onClick={() => setReloadKey((value) => value + 1)}>{t("actions.retry")}</Button>}
            severity="error"
          >
            {loadError || t("shares.convert.loadFailed")}
          </Alert>
        ) : null}
        {publicURL ? (
          <div className="grid gap-1">
            <Typography color="text.secondary" variant="subtitle2">{t("shares.convert.generatedUrl")}</Typography>
            <Typography
              className="block max-h-40 cursor-text overflow-auto break-words rounded border border-divider p-3 select-text [overflow-wrap:anywhere]"
              component="code"
              ref={generatedURL}
              onClick={(event) => {
                if (!hasSelectionWithin(event.currentTarget)) selectContents(event.currentTarget);
              }}
            >
              {publicURL}
            </Typography>
          </div>
        ) : null}

        <DialogActions className="px-0 pb-0">
          <Button type="button" onClick={onClose}>{t("actions.cancel")}</Button>
          <Button disabled={!publicURL || loading || loadError !== null} type="button" variant="contained" onClick={() => { void copyLink(); }}>
            {t("shares.convert.copy")}
          </Button>
        </DialogActions>
      </Stack>
    </AppDialog>
  );
}

function formatNames(capabilities: FormatCapabilitySummary[], direction: FormatCapabilityDirection): string[] {
  const seen = new Set<string>();
  const formats: string[] = [];
  for (const capability of capabilities) {
    const format = capability.format.trim();
    if (capability.direction !== direction || !format || seen.has(format)) continue;
    seen.add(format);
    formats.push(format);
  }
  return formats;
}

function validationMessage(error: ConvertLinkValidationError | null, t: (key: TranslationKey, params?: Record<string, string | number>) => string): string {
  switch (error) {
    case "source_required":
      return t("shares.convert.validation.sourceRequired");
    case "url_invalid":
      return t("shares.convert.validation.urlInvalid");
    case "url_scheme":
      return t("shares.convert.validation.urlScheme");
    case "content_too_large":
      return t("shares.convert.validation.contentTooLarge");
    case "to_format_required":
      return t("shares.convert.validation.toFormatRequired");
    default:
      return "";
  }
}
