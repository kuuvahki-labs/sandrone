import { useEffect, useRef, useState } from "react";
import Alert from "@mui/material/Alert";
import Button from "@mui/material/Button";
import CircularProgress from "@mui/material/CircularProgress";
import DialogActions from "@mui/material/DialogActions";
import Divider from "@mui/material/Divider";
import Typography from "@mui/material/Typography";

import type { NodeIPInfo, NodeURIResult, SubscriptionPreviewNode } from "~/features/subscriptions/model/types";
import { useI18n } from "~/shared/i18n/context";
import { CollapsibleWarningPanel } from "~/shared/resources/warning-panel";
import { AppDialog } from "~/shared/ui/dialogs";
import { QrCodePanel } from "~/shared/ui/qr-code";
import { hasSelectionWithin, selectContents } from "~/shared/ui/text-transfer";

export function NodeInfoDialog({
  node,
  onClose,
  onCopyURI,
  onLookupIP,
  onRenderURI,
}: {
  node: SubscriptionPreviewNode;
  onClose: () => void;
  onCopyURI: (uri: string) => Promise<boolean>;
  onLookupIP: (node: SubscriptionPreviewNode) => Promise<NodeIPInfo>;
  onRenderURI: (node: SubscriptionPreviewNode) => Promise<NodeURIResult>;
}) {
  const { t } = useI18n();
  const [error, setError] = useState("");
  const [reloadKey, setReloadKey] = useState(0);
  const [result, setResult] = useState<NodeURIResult>();
  const [sharingError, setSharingError] = useState("");
  const [ipInfo, setIPInfo] = useState<NodeIPInfo>();
  const [ipError, setIPError] = useState("");
  const [ipLoading, setIPLoading] = useState(false);
  const [ipReloadKey, setIPReloadKey] = useState(0);
  const uriElement = useRef<HTMLElement | null>(null);
  const canShare = typeof navigator.share === "function";

  useEffect(() => {
    let active = true;
    setResult(undefined);
    setError("");
    void onRenderURI(node).then((next) => {
      if (active) setResult(next);
    }).catch((renderError: unknown) => {
      if (active) setError(renderError instanceof Error ? renderError.message : t("subscriptions.nodeTools.renderFailed"));
    });
    return () => {
      active = false;
    };
  }, [node, onRenderURI, reloadKey, t]);

  useEffect(() => {
    let active = true;
    setIPInfo(undefined);
    setIPError("");
    setIPLoading(true);
    void onLookupIP(node).then((next) => {
      if (active) setIPInfo(next);
    }).catch((lookupError: unknown) => {
      if (active) setIPError(lookupError instanceof Error ? lookupError.message : t("subscriptions.nodeTools.ip.failed"));
    }).finally(() => {
      if (active) setIPLoading(false);
    });
    return () => {
      active = false;
    };
  }, [ipReloadKey, node, onLookupIP, t]);

  async function copyURI() {
    if (result && !(await onCopyURI(result.uri))) {
      selectContents(uriElement.current);
    }
  }

  async function shareURI() {
    if (!result || !navigator.share) return;
    setSharingError("");
    try {
      await navigator.share({ text: result.uri, title: node.name });
    } catch (shareError) {
      if (!(shareError instanceof DOMException && shareError.name === "AbortError")) {
        setSharingError(t("subscriptions.nodeTools.shareFailed"));
      }
    }
  }

  return (
    <AppDialog title={t("subscriptions.nodeTools.title")} onClose={onClose}>
      <div className="grid min-w-0 gap-4">
        <Typography className="break-words [overflow-wrap:anywhere]" component="p" variant="subtitle1">
          {node.name || t("subscriptions.preview.unnamedNode")}
        </Typography>
        <section aria-labelledby="node-ip-info-heading" className="grid gap-3">
          <Typography component="h3" id="node-ip-info-heading" variant="h6">
            {t("subscriptions.nodeTools.ip.title")}
          </Typography>
          {ipLoading ? (
            <div className="flex items-center gap-3" role="status">
              <CircularProgress size={20} />
              <Typography color="text.secondary">{t("subscriptions.nodeTools.ip.loading")}</Typography>
            </div>
          ) : null}
          {ipError ? (
            <Alert
              action={<Button color="inherit" size="small" onClick={() => setIPReloadKey((value) => value + 1)}>{t("actions.retry")}</Button>}
              severity="error"
            >
              {ipError}
            </Alert>
          ) : null}
          {ipInfo ? <NodeIPInfoView info={ipInfo} /> : null}
        </section>
        <Divider />
        {!result && !error ? (
          <div className="flex items-center gap-3" role="status">
            <CircularProgress size={20} />
            <Typography color="text.secondary">{t("subscriptions.nodeTools.rendering")}</Typography>
          </div>
        ) : null}
        {error ? (
          <Alert
            action={<Button color="inherit" size="small" onClick={() => setReloadKey((value) => value + 1)}>{t("actions.retry")}</Button>}
            severity="error"
          >
            {error}
          </Alert>
        ) : null}
        {result ? (
          <>
            {result.warnings.length ? (
              <CollapsibleWarningPanel label={t("subscriptions.nodeTools.warnings")} warnings={result.warnings} />
            ) : null}
            <QrCodePanel label={t("subscriptions.nodeTools.qrcode", { name: node.name })} value={result.uri} />
            <div className="grid gap-1">
              <Typography color="text.secondary" component="span" variant="body2">
                {t("subscriptions.nodeTools.uri")}
              </Typography>
              <Typography
                className="block cursor-text break-words select-text [overflow-wrap:anywhere]"
                component="code"
                ref={uriElement}
                variant="body2"
                onClick={(event) => {
                  if (!hasSelectionWithin(event.currentTarget)) selectContents(event.currentTarget);
                }}
              >
                {result.uri}
              </Typography>
            </div>
          </>
        ) : null}
        {sharingError ? <Alert severity="error">{sharingError}</Alert> : null}
      </div>
      <DialogActions className="px-0 pb-0">
        <Button type="button" onClick={onClose}>{t("actions.close")}</Button>
        {canShare && result ? <Button type="button" onClick={() => { void shareURI(); }}>{t("subscriptions.nodeTools.systemShare")}</Button> : null}
        {result ? <Button type="button" variant="contained" onClick={() => { void copyURI(); }}>{t("subscriptions.nodeTools.copy")}</Button> : null}
      </DialogActions>
    </AppDialog>
  );
}

function NodeIPInfoView({ info }: { info: NodeIPInfo }) {
  const { t } = useI18n();
  if (!info.public) {
    return <Alert severity="info">{t("subscriptions.nodeTools.ip.nonPublic", { ip: info.ip })}</Alert>;
  }
  const location = [
    info.country ? `${info.country}${info.countryCode ? ` (${info.countryCode})` : ""}` : "",
    info.continent ? `${info.continent}${info.continentCode ? ` (${info.continentCode})` : ""}` : "",
  ].filter(Boolean).join(" · ");
  const network = [info.asn, info.asName, info.asDomain].filter(Boolean).join(" · ");
  return (
    <dl className="grid grid-cols-[auto_minmax(0,1fr)] gap-x-4 gap-y-2 text-sm">
      <dt className="text-text-secondary">{t("subscriptions.nodeTools.ip.address")}</dt>
      <dd className="m-0 break-all font-mono">{info.ip}</dd>
      <dt className="text-text-secondary">{t("subscriptions.nodeTools.ip.location")}</dt>
      <dd className="m-0">{location}</dd>
      <dt className="text-text-secondary">{t("subscriptions.nodeTools.ip.network")}</dt>
      <dd className="m-0 break-words">{network}</dd>
      {info.source ? (
        <>
          <dt className="text-text-secondary">{t("subscriptions.nodeTools.ip.source")}</dt>
          <dd className="m-0">
            <a className="text-primary underline" href={info.source.url} rel="noreferrer" target="_blank">{info.source.name}</a>
          </dd>
        </>
      ) : null}
    </dl>
  );
}
