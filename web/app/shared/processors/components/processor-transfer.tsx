import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import ContentCopyOutlinedIcon from "@mui/icons-material/ContentCopyOutlined";
import ContentPasteOutlinedIcon from "@mui/icons-material/ContentPasteOutlined";
import UploadFileOutlinedIcon from "@mui/icons-material/UploadFileOutlined";
import Alert from "@mui/material/Alert";
import Button from "@mui/material/Button";
import IconButton from "@mui/material/IconButton";
import TextField from "@mui/material/TextField";
import Tooltip from "@mui/material/Tooltip";
import Typography from "@mui/material/Typography";

import { useI18n } from "~/shared/i18n/context";
import { parseProcessorTransfer, ProcessorTransferError, serializeProcessorTransfer } from "~/shared/processors/transfer";
import type { ProcessorDetail } from "~/shared/resources/types";
import { AppDialog } from "~/shared/ui/dialogs";
import { SnackbarStack } from "~/shared/ui/feedback";
import { copyText } from "~/shared/ui/text-transfer";

type TransferDialog = { mode: "copy" | "import"; text: string; error: string };

export function ProcessorTransfer({ actionsContainer, processors, onImport }: {
  actionsContainer?: HTMLElement | null;
  processors: ProcessorDetail[];
  onImport: (processors: ProcessorDetail[]) => void;
}) {
  const { t } = useI18n();
  const [dialog, setDialog] = useState<TransferDialog | null>(null);
  const [pending, setPending] = useState(false);
  const [notice, setNotice] = useState("");
  const busy = useRef(false);
  const mounted = useRef(false);
  const importRef = useRef(onImport);

  useLayoutEffect(() => { importRef.current = onImport; }, [onImport]);
  useLayoutEffect(() => {
    mounted.current = true;
    return () => { mounted.current = false; };
  }, []);
  useEffect(() => {
    if (!notice) return;
    const timer = window.setTimeout(() => setNotice(""), 2600);
    return () => window.clearTimeout(timer);
  }, [notice]);

  function start() {
    if (busy.current) return false;
    busy.current = true;
    setPending(true);
    return true;
  }

  function finish() {
    busy.current = false;
    if (mounted.current) setPending(false);
  }

  function errorMessage(error: unknown) {
    return t(error instanceof ProcessorTransferError && error.code === "invalid_json"
      ? "processors.transfer.invalidJson"
      : "processors.transfer.invalidProcessors");
  }

  function importText(text: string) {
    const imported = parseProcessorTransfer(text);
    importRef.current(imported);
    setDialog(null);
    setNotice(t("processors.transfer.imported", { count: imported.length }));
  }

  async function copyProcessors() {
    if (!start()) return;
    const text = serializeProcessorTransfer(processors);
    try {
      const copied = await copyText(text);
      if (!mounted.current) return;
      if (copied) setNotice(t("processors.transfer.copied"));
      else setDialog({ mode: "copy", text, error: "" });
    } finally {
      finish();
    }
  }

  async function importProcessors() {
    if (!start()) return;
    let text = "";
    try {
      if (!navigator.clipboard?.readText) {
        setDialog({ mode: "import", text, error: "" });
        return;
      }
      try {
        text = await navigator.clipboard.readText();
      } catch {
        if (mounted.current) setDialog({ mode: "import", text, error: "" });
        return;
      }
      if (!mounted.current) return;
      try {
        importText(text);
      } catch (error) {
        setDialog({ mode: "import", text, error: errorMessage(error) });
      }
    } finally {
      finish();
    }
  }

  async function importFile(file: File | undefined) {
    if (!file || !start()) return;
    try {
      const text = await file.text();
      if (mounted.current) importText(text);
    } catch (error) {
      if (mounted.current) setDialog((current) => current ? { ...current, error: errorMessage(error) } : current);
    } finally {
      finish();
    }
  }

  function confirmImport() {
    if (!dialog || !start()) return;
    try {
      importText(dialog.text);
    } catch (error) {
      setDialog({ ...dialog, error: errorMessage(error) });
    } finally {
      finish();
    }
  }

  const actions = (
    <span className="inline-flex shrink-0 items-center gap-1">
      <Tooltip title={t("actions.copy")}>
        <span><IconButton aria-label={t("processors.transfer.copy")} disabled={pending || processors.length === 0} size="small" type="button" onClick={() => { void copyProcessors(); }}>
          <ContentCopyOutlinedIcon aria-hidden fontSize="small" />
        </IconButton></span>
      </Tooltip>
      <Tooltip title={t("actions.import")}>
        <span><IconButton aria-label={t("processors.transfer.import")} disabled={pending} size="small" type="button" onClick={() => { void importProcessors(); }}>
          <ContentPasteOutlinedIcon aria-hidden fontSize="small" />
        </IconButton></span>
      </Tooltip>
    </span>
  );

  const content = (
    <>
      {actions}
      {dialog ? (
        <AppDialog disableClose={pending} title={t(dialog.mode === "copy" ? "processors.transfer.copy" : "processors.transfer.import")} onClose={() => setDialog(null)}>
          <div className="grid gap-4">
            <Typography color="text.secondary" variant="body2">
              {t(dialog.mode === "copy" ? "processors.transfer.manualCopy" : "processors.transfer.manualImport")}
            </Typography>
            {dialog.error ? <Alert severity="error">{dialog.error}</Alert> : null}
            <TextField
              fullWidth
              multiline
              disabled={pending}
              label={t("processors.transfer.json")}
              maxRows={12}
              minRows={6}
              slotProps={{ input: { readOnly: dialog.mode === "copy" } }}
              value={dialog.text}
              onChange={(event) => setDialog({ ...dialog, text: event.target.value, error: "" })}
              onFocus={(event) => { if (dialog.mode === "copy") event.target.select(); }}
            />
            <div className="flex flex-wrap justify-end gap-2">
              {dialog.mode === "import" ? (
                <Button component="label" disabled={pending} startIcon={<UploadFileOutlinedIcon aria-hidden />} variant="outlined">
                  {t("processors.transfer.selectFile")}
                  <input
                    accept=".json,application/json"
                    aria-label={t("processors.transfer.selectFile")}
                    className="sr-only"
                    disabled={pending}
                    type="file"
                    onChange={(event) => {
                      void importFile(event.currentTarget.files?.[0]);
                      event.currentTarget.value = "";
                    }}
                  />
                </Button>
              ) : null}
              <Button disabled={pending} type="button" onClick={() => setDialog(null)}>{t("actions.close")}</Button>
              {dialog.mode === "import" ? (
                <Button disabled={pending || !dialog.text.trim()} type="button" variant="contained" onClick={confirmImport}>{t("actions.import")}</Button>
              ) : null}
            </div>
          </div>
        </AppDialog>
      ) : null}
      <SnackbarStack notices={notice ? [{ id: 0, message: notice, severity: "success" }] : []} />
    </>
  );
  return actionsContainer === undefined ? content : actionsContainer ? createPortal(content, actionsContainer) : null;
}
