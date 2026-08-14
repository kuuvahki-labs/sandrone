import { type ReactNode, useEffect, useId, useMemo, useState } from "react";
import AccountTreeOutlinedIcon from "@mui/icons-material/AccountTreeOutlined";
import CloudDownloadOutlinedIcon from "@mui/icons-material/CloudDownloadOutlined";
import ContentCopyIcon from "@mui/icons-material/ContentCopy";
import LinkOutlinedIcon from "@mui/icons-material/LinkOutlined";
import Button from "@mui/material/Button";
import FormControl from "@mui/material/FormControl";
import IconButton from "@mui/material/IconButton";
import InputAdornment from "@mui/material/InputAdornment";
import InputLabel from "@mui/material/InputLabel";
import Paper from "@mui/material/Paper";
import Select from "@mui/material/Select";
import TextField from "@mui/material/TextField";
import Tooltip from "@mui/material/Tooltip";
import Typography from "@mui/material/Typography";

import type { SubscriptionDefinition, SubscriptionItem } from "~/features/subscriptions/model/types";
import type { ProbeDefaultsInput } from "~/shared/api/client";
import { useI18n } from "~/shared/i18n/context";
import type { ResourceOption } from "~/shared/resources/types";
import type { SubscriptionCreateType } from "~/shared/routing/paths";
import { HighlightedTextarea } from "~/shared/ui/code-editor";
import { RenderCachePolicyField } from "~/shared/ui/render-cache-policy-field";

import { ProcessorBuilder } from "./processor-builder";
import { SourceMultiSelect } from "./source-multi-select";

export interface SubscriptionFormFieldsProps {
  definition?: SubscriptionDefinition | null;
  item?: SubscriptionItem;
  mode: "create" | "edit";
  onCopySource?: (value: string, target: SubscriptionCopyTarget) => void | Promise<void>;
  onDirty?: () => void;
  onTypeChange: (type: SubscriptionCreateType) => void;
  probeCacheTTLSeconds: number;
  probeDefaults: ProbeDefaultsInput;
  scriptFiles?: ResourceOption[];
  sources: SubscriptionItem[];
  type: SubscriptionCreateType;
}

export type SubscriptionCopyTarget = "content" | "url";

const emptySourceRefs: string[] = [];

export function SubscriptionFormFields({ definition, item, mode, onCopySource, onDirty, onTypeChange, probeCacheTTLSeconds, probeDefaults, scriptFiles, sources, type }: SubscriptionFormFieldsProps) {
  const { t } = useI18n();
  const meta = definition?.meta ?? (item?.description ? { description: item.description } : {});
  const description = meta.description ?? item?.description ?? "";
  const displayName = definition?.displayName ?? item?.displayName ?? "";
  const processorDefaultValue = definition?.processors;
  const originalType = item?.kind;
  const useOriginalTypeFields = !item || type === originalType;
  const remote = useOriginalTypeFields ? definition?.remote : undefined;
  const sourceInput = type === "remote" ? remote?.url ?? "" : type === "local" && useOriginalTypeFields ? definition?.content ?? "" : "";
  const [collectionName, setCollectionName] = useState(() => defaultNameForType({ item, mode, type }));
  const [sourceInputValue, setSourceInputValue] = useState(sourceInput);
  const collectionDefaultValue = useMemo(() => {
    if (!item) return emptySourceRefs;
    if (type !== item.kind) return emptySourceRefs;
    if (definition) return definition.sourceRefs;
    return undefined;
  }, [definition, item, type]);

  useEffect(() => {
    setCollectionName(defaultNameForType({ item, mode, type }));
  }, [item, mode, type]);

  useEffect(() => {
    setSourceInputValue(sourceInput);
  }, [item?.kind, item?.name, mode, sourceInput, type]);

  function copySourceValue(target: SubscriptionCopyTarget) {
    void onCopySource?.(sourceInputValue, target);
  }

  return (
    <div className="grid gap-4">
      <Paper className="m-0 min-w-0 p-4" component="fieldset" variant="outlined">
        <Typography className="px-1 font-semibold" component="legend">
          {mode === "create" ? t("subscriptions.form.createType") : t("subscriptions.form.subscriptionType")}
        </Typography>
        <SubscriptionTypeOptions active={type} ariaLabel={mode === "create" ? t("subscriptions.form.createType") : t("subscriptions.form.subscriptionType")} onSelect={onTypeChange} />
      </Paper>
      <Paper className="m-0 min-w-0 p-4" component="fieldset" variant="outlined">
        <Typography className="px-1 font-semibold" component="legend">
          {type === "collection" ? t("subscriptions.form.collectionInfo") : t("subscriptions.form.basicInfo")}
        </Typography>
        <div className="grid gap-4 md:grid-cols-2">
          <input name="subscription_type" type="hidden" value={type} />
          {type === "remote" ? (
            <>
              <TextField disabled={mode === "edit"} fullWidth defaultValue={defaultNameForType({ item, mode, type })} label={t("labels.name")} name="name" placeholder={t("subscriptions.form.namePlaceholder")} />
              <TextField fullWidth defaultValue={displayName} label={t("subscriptions.form.displayName")} name="display_name" />
              <TextField fullWidth multiline className="md:col-span-2" defaultValue={description} label={t("subscriptions.form.description")} minRows={2} name="description" />
              <TextField
                fullWidth
                className="md:col-span-2"
                label={t("subscriptions.form.url")}
                name="source_input"
                placeholder="https://example.com/sub"
                value={sourceInputValue}
                slotProps={{
                  input: onCopySource ? {
                    endAdornment: (
                      <InputAdornment position="end">
                        <Tooltip title={t("actions.copy")}>
                          <IconButton aria-label={t("subscriptions.form.copyUrl")} edge="end" size="small" type="button" onClick={() => copySourceValue("url")}>
                            <ContentCopyIcon aria-hidden fontSize="small" />
                          </IconButton>
                        </Tooltip>
                      </InputAdornment>
                    ),
                  } : undefined,
                }}
                onChange={(event) => setSourceInputValue(event.target.value)}
              />
              <SubscriptionFormatField defaultValue={formatDefaultValue(type, item, definition)} />
              <TextField fullWidth defaultValue={remote?.user_agent ?? ""} label="User-Agent" name="user_agent" />
              <TextField fullWidth defaultValue={remote?.proxy ?? ""} label={t("subscriptions.form.proxy")} name="proxy" placeholder="http://127.0.0.1:7890" />
              <TextField fullWidth defaultValue={remote?.timeout_ms ?? ""} label={t("subscriptions.form.timeoutMs")} name="timeout_ms" type="number" />
              <TextField fullWidth defaultValue={remote?.cache_ttl_seconds ?? ""} label={t("cache.remoteFetchTTLSeconds")} name="cache_ttl_seconds" type="number" />
              {mode === "edit" ? <input name="meta" type="hidden" defaultValue={formatJSONForForm(meta)} /> : null}
            </>
          ) : null}
          {type === "local" ? (
            <>
              <TextField disabled={mode === "edit"} fullWidth defaultValue={defaultNameForType({ item, mode, type })} label={t("labels.name")} name="name" />
              <TextField fullWidth defaultValue={displayName} label={t("subscriptions.form.displayName")} name="display_name" />
              <TextField fullWidth multiline className="md:col-span-2" defaultValue={description} label={t("subscriptions.form.description")} minRows={2} name="description" />
              <HighlightedTextarea
                showLineNumbers
                className="md:col-span-2"
                label={t("subscriptions.form.content")}
                labelAction={onCopySource ? (
                  <Tooltip title={t("actions.copy")}>
                    <IconButton aria-label={t("subscriptions.form.copyContent")} size="small" type="button" onClick={() => copySourceValue("content")}>
                      <ContentCopyIcon aria-hidden fontSize="small" />
                    </IconButton>
                  </Tooltip>
                ) : undefined}
                language="text"
                minRows={6}
                name="source_input"
                placeholder={"ss://...\nvmess://..."}
                value={sourceInputValue}
                onChange={(event) => setSourceInputValue(event.target.value)}
              />
              <SubscriptionFormatField defaultValue={formatDefaultValue(type, item, definition)} />
              {mode === "edit" ? <input name="meta" type="hidden" defaultValue={formatJSONForForm(meta)} /> : null}
            </>
          ) : null}
          {type === "collection" ? (
            <div className="md:col-span-2 grid gap-4">
              <TextField disabled={mode === "edit"} fullWidth label={t("labels.name")} name="name" value={collectionName} onChange={(event) => setCollectionName(event.target.value)} />
              <TextField fullWidth defaultValue={displayName} label={t("subscriptions.form.displayName")} name="display_name" />
              <TextField fullWidth multiline defaultValue={description} label={t("subscriptions.form.description")} minRows={2} name="description" />
              {mode === "edit" ? <input name="meta" type="hidden" defaultValue={formatJSONForForm(meta)} /> : null}
              <SourceMultiSelect defaultValue={collectionDefaultValue} excludeName={collectionName} onDirty={onDirty} subscriptions={sources} />
            </div>
          ) : null}
          <RenderCachePolicyField defaultValue={definition?.renderCacheTTLSeconds} />
        </div>
      </Paper>
      <Paper className="m-0 min-w-0 p-4" component="fieldset" variant="outlined">
        <Typography className="px-1 font-semibold" component="legend">
          {t("subscriptions.form.processors")}
        </Typography>
        <ProcessorBuilder defaultValue={processorDefaultValue} onDirty={onDirty} probeCacheTTLSeconds={probeCacheTTLSeconds} probeDefaults={probeDefaults} scriptFiles={scriptFiles} />
      </Paper>
    </div>
  );
}

function SubscriptionTypeOptions({ active, ariaLabel, onSelect }: { active: SubscriptionCreateType; ariaLabel: string; onSelect: (type: SubscriptionCreateType) => void }) {
  const { t } = useI18n();
  const options: Array<{ description: string; icon: ReactNode; label: string; value: SubscriptionCreateType }> = [
    { description: t("subscriptions.type.remote.description"), icon: <CloudDownloadOutlinedIcon aria-hidden fontSize="small" />, label: t("model.subscription.remoteShort"), value: "remote" },
    { description: t("subscriptions.type.local.description"), icon: <LinkOutlinedIcon aria-hidden fontSize="small" />, label: t("model.subscription.localShort"), value: "local" },
    { description: t("subscriptions.type.collection.description"), icon: <AccountTreeOutlinedIcon aria-hidden fontSize="small" />, label: t("model.subscription.collectionShort"), value: "collection" },
  ];

  return (
    <div aria-label={ariaLabel} className="grid gap-2 sm:grid-cols-3">
      {options.map((option) => (
        <Button
          aria-label={option.label}
          aria-pressed={active === option.value}
          fullWidth
          key={option.value}
          startIcon={option.icon}
          type="button"
          variant={active === option.value ? "contained" : "outlined"}
          onClick={() => onSelect(option.value)}
        >
          <span className="grid justify-items-start">
            <Typography component="strong" variant="body2">{option.label}</Typography>
            <Typography color="text.secondary" variant="caption">{option.description}</Typography>
          </span>
        </Button>
      ))}
    </div>
  );
}

function SubscriptionFormatField({ defaultValue }: { defaultValue?: string }) {
  const { t } = useI18n();
  const labelId = useId();
  const selectId = useId();
  return (
    <FormControl fullWidth>
      <InputLabel htmlFor={selectId} id={labelId}>{t("subscriptions.form.format")}</InputLabel>
      <Select native defaultValue={formatSelectValue(defaultValue)} id={selectId} inputProps={{ name: "format" }} label={t("subscriptions.form.format")} labelId={labelId}>
        <option value="auto">{t("subscriptions.form.formatAuto")}</option>
        <option value="uri">URI</option>
        <option value="uri-list">URI List</option>
        <option value="base64">Base64</option>
        <option value="mihomo">Mihomo</option>
        <option value="sing-box">sing-box</option>
      </Select>
    </FormControl>
  );
}

function defaultNameForType({ item, mode, type }: { item?: SubscriptionItem; mode: "create" | "edit"; type: SubscriptionCreateType }): string {
  if (mode === "edit") return item?.name ?? "";
  if (type === "local") return "manual";
  if (type === "collection") return "default";
  return "";
}

function formatDefaultValue(type: SubscriptionCreateType, item?: SubscriptionItem, definition?: SubscriptionDefinition | null): string | undefined {
  if (type === "collection") return undefined;
  if (item && item.kind !== type && item.kind === "collection") return undefined;
  return definition?.format ?? item?.format;
}

function formatSelectValue(format?: string): string {
  const value = String(format ?? "").trim();
  return value && value.toLowerCase() !== "auto" ? value : "auto";
}

function formatJSONForForm(value: unknown): string {
  if (value === undefined || value === null) {
    return "";
  }
  if (Array.isArray(value) && value.length === 0) {
    return "";
  }
  if (!Array.isArray(value) && typeof value === "object" && Object.keys(value).length === 0) {
    return "";
  }
  return JSON.stringify(value, null, 2);
}
