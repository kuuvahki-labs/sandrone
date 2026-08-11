import { useEffect, useRef, useState } from "react";
import WarningAmberOutlinedIcon from "@mui/icons-material/WarningAmberOutlined";
import Alert from "@mui/material/Alert";
import Autocomplete from "@mui/material/Autocomplete";
import Button from "@mui/material/Button";
import Collapse from "@mui/material/Collapse";
import List from "@mui/material/List";
import ListItem from "@mui/material/ListItem";
import TextField from "@mui/material/TextField";
import Typography from "@mui/material/Typography";

import {
  type ConfigNodePreview,
  configNodePreviewFromSubscription,
  type ConfigNodePreviewInput,
} from "~/features/files/config/model/node-source";
import { useI18n } from "~/shared/i18n/context";
import { resourceOptionText } from "~/shared/resources/labels";
import type { ResourceOption } from "~/shared/resources/types";
import { WarningList } from "~/shared/resources/warnings";

import { WorkbenchGroupSection } from "./editor-shared";

export type LoadSubscriptionPreview = (name: string) => Promise<ConfigNodePreviewInput>;

export type ConfigNodeSourceState =
  | { status: "idle"; subscriptionName: ""; preview: null }
  | { status: "loading"; subscriptionName: string; preview: null }
  | { status: "ready"; subscriptionName: string; preview: ConfigNodePreview }
  | { status: "error"; subscriptionName: string; preview: null; error: string };

export interface ConfigNodeSourceSectionProps {
  disabled?: boolean;
  loadPreview: LoadSubscriptionPreview;
  onSelectedChange: (name: string) => void;
  onStateChange?: (state: ConfigNodeSourceState) => void;
  selected: string;
  subscriptions: ResourceOption[];
}

interface PreviewRequest {
  promise: Promise<ConfigNodePreview>;
  version: number;
}

export function ConfigNodeSourceSection({ disabled = false, loadPreview, onSelectedChange, onStateChange, selected, subscriptions }: ConfigNodeSourceSectionProps) {
  const { t } = useI18n();
  const cache = useRef(new Map<string, ConfigNodePreview>());
  const inFlight = useRef(new Map<string, PreviewRequest>());
  const requestVersions = useRef(new Map<string, number>());
  const [error, setError] = useState("");
  const [nodesExpanded, setNodesExpanded] = useState(false);
  const [pending, setPending] = useState(false);
  const [preview, setPreview] = useState<ConfigNodePreview | null>(null);
  const [revision, setRevision] = useState(0);
  const previousRevision = useRef(revision);
  const stateChangeRef = useRef(onStateChange);
  const selectedItem = subscriptions.find((item) => item.name === selected) ?? null;
  const mismatchError = t("files.config.nodeSourceMismatch");

  useEffect(() => {
    stateChangeRef.current = onStateChange;
  }, [onStateChange]);

  useEffect(() => {
    if (!selected) {
      setError("");
      setNodesExpanded(false);
      setPending(false);
      setPreview(null);
      stateChangeRef.current?.({ status: "idle", subscriptionName: "", preview: null });
      return;
    }
    const force = previousRevision.current !== revision;
    previousRevision.current = revision;
    const pendingRequest = inFlight.current.get(selected);
    if (force) cache.current.delete(selected);
    const cached = cache.current.get(selected);
    if (cached && !force && !pendingRequest) {
      setError("");
      setPending(false);
      setPreview(cached);
      stateChangeRef.current?.({ status: "ready", subscriptionName: selected, preview: cached });
      return;
    }
    let active = true;
    setError("");
    setNodesExpanded(false);
    setPending(true);
    setPreview(null);
    stateChangeRef.current?.({ status: "loading", subscriptionName: selected, preview: null });
    let request = force ? undefined : pendingRequest;
    if (!request) {
      const version = (requestVersions.current.get(selected) ?? 0) + 1;
      requestVersions.current.set(selected, version);
      const promise = loadPreview(selected).then((response) => {
        if (response.subscriptionName !== selected) throw new Error(mismatchError);
        return configNodePreviewFromSubscription(response);
      });
      request = { promise, version };
      inFlight.current.set(selected, request);
      const currentRequest = request;
      const clearRequest = () => {
        if (inFlight.current.get(selected) === currentRequest) inFlight.current.delete(selected);
      };
      void promise.then(clearRequest, clearRequest);
    }
    const currentRequest = request;
    void currentRequest.promise.then((nextPreview) => {
      if (requestVersions.current.get(selected) !== currentRequest.version) return;
      cache.current.set(selected, nextPreview);
      if (active) {
        setPreview(nextPreview);
        stateChangeRef.current?.({ status: "ready", subscriptionName: selected, preview: nextPreview });
      }
    }).catch((loadError: unknown) => {
      if (active && requestVersions.current.get(selected) === currentRequest.version) {
        const message = loadError instanceof Error ? loadError.message : String(loadError);
        setPreview(null);
        setError(message);
        stateChangeRef.current?.({ status: "error", subscriptionName: selected, preview: null, error: message });
      }
    }).finally(() => {
      if (active && requestVersions.current.get(selected) === currentRequest.version) setPending(false);
    });
    return () => { active = false; };
  }, [loadPreview, mismatchError, revision, selected]);

  return (
    <WorkbenchGroupSection collapsible={false} id="config-node-source" label={t("files.config.nodeSource")}>
      <Autocomplete
        autoHighlight
        disabled={disabled}
        options={subscriptions}
        value={selectedItem}
        getOptionLabel={resourceOptionText}
        isOptionEqualToValue={(option, value) => option.name === value.name}
        renderInput={(params) => <TextField {...params} label={t("files.config.subscription")} />}
        onChange={(_event, item) => onSelectedChange(item?.name ?? "")}
      />
      {pending ? <Typography color="text.secondary" role="status" variant="body2">{t("files.config.nodeSourceLoading")}</Typography> : null}
      {error ? (
        <Alert
          action={<Button aria-label={t("files.config.nodeSourceRetry")} color="inherit" size="small" type="button" onClick={() => setRevision((current) => current + 1)}>{t("actions.retry")}</Button>}
          role="alert"
          severity="warning"
          variant="outlined"
        >
          {error}
        </Alert>
      ) : null}
      {preview ? (
        <div className="grid gap-2">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <Typography color="text.secondary" role="status" variant="body2">
              {preview.warnings.length
                ? t("files.config.nodeSourceLoadedWithWarnings", { count: preview.nodes.length, warningCount: preview.warnings.length })
                : t("files.config.nodeSourceLoaded", { count: preview.nodes.length })}
            </Typography>
            <div className="flex flex-wrap gap-1">
              <Button aria-label={t("files.config.nodeSourceRefresh")} size="small" type="button" onClick={() => setRevision((current) => current + 1)}>{t("actions.refresh")}</Button>
              <Button
                aria-expanded={nodesExpanded}
                size="small"
                type="button"
                onClick={() => setNodesExpanded((current) => !current)}
              >
                {t(nodesExpanded ? "files.config.nodeSourceCollapse" : "files.config.nodeSourceExpand")}
              </Button>
            </div>
          </div>
          <Collapse in={nodesExpanded} timeout="auto" unmountOnExit>
            <div className="grid gap-3">
              {preview.warnings.length ? (
                <section aria-labelledby="config-node-source-warnings" className="grid gap-2">
                  <Typography className="flex items-center gap-1 font-semibold" color="warning.main" component="h4" id="config-node-source-warnings" variant="body2">
                    <WarningAmberOutlinedIcon aria-hidden fontSize="small" />
                    {t("files.config.nodeSourceWarnings", { count: preview.warnings.length })}
                  </Typography>
                  <WarningList className="max-h-[min(30vh,12rem)] overflow-y-auto" warnings={preview.warnings} />
                </section>
              ) : null}
              {preview.duplicateNames.length || preview.unnamedCount ? (
                <div className="grid max-h-[min(30vh,12rem)] gap-1 overflow-y-auto rounded-md border border-warning-main/40 px-3 py-2">
                  {preview.duplicateNames.length ? <Typography color="warning.main" variant="caption">{t("files.config.nodeSourceDuplicateNames", { names: preview.duplicateNames.join(", ") })}</Typography> : null}
                  {preview.unnamedCount ? <Typography color="warning.main" variant="caption">{t("files.config.nodeSourceUnnamed", { count: preview.unnamedCount })}</Typography> : null}
                </div>
              ) : null}
              <List className="p-0">
                {preview.nodes.map((node) => (
                  <ListItem className="grid min-w-0 gap-1 px-0" key={node.key}>
                    <Typography className="break-words" component="span" variant="body2">{node.name}</Typography>
                    <Typography className="break-words" color="text.secondary" component="span" variant="caption">{node.type ?? "-"} · {node.endpoint}</Typography>
                  </ListItem>
                ))}
              </List>
            </div>
          </Collapse>
        </div>
      ) : null}
    </WorkbenchGroupSection>
  );
}
