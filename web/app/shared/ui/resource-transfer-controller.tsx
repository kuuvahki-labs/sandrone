import { type ReactNode, useRef, useState } from "react";
import ContentCopyOutlinedIcon from "@mui/icons-material/ContentCopyOutlined";
import UploadFileOutlinedIcon from "@mui/icons-material/UploadFileOutlined";
import Alert from "@mui/material/Alert";
import Button from "@mui/material/Button";
import Dialog from "@mui/material/Dialog";
import DialogActions from "@mui/material/DialogActions";
import DialogContent from "@mui/material/DialogContent";
import DialogContentText from "@mui/material/DialogContentText";
import DialogTitle from "@mui/material/DialogTitle";
import TextField from "@mui/material/TextField";
import Typography from "@mui/material/Typography";

import { type Translator } from "~/shared/i18n/context";
import {
  copyTransferredResource,
  parseResourceTransfer,
  renameTransferredResource,
  resourceNameIssue,
  type ResourceTransfer,
  ResourceTransferError,
  resourceTransferFilename,
  serializeResourceTransfer,
  type TransferResourceType,
} from "~/shared/resources/resource-transfer";

type Notice = (message: string, severity?: "success" | "error" | "warning") => void;

export interface ResourceTransferControllerOptions {
  existingNames: readonly string[];
  loadResource: (name: string) => Promise<unknown>;
  onSaved: (resource: Record<string, unknown>) => void | Promise<void>;
  resourceType: TransferResourceType;
  saveResource: (resource: Record<string, unknown>) => Promise<unknown>;
  showNotice: Notice;
  t: Translator;
}

interface CopyState {
  error: string;
  name: string;
  pending: boolean;
  sourceName: string;
}

interface ImportState {
  error: string;
  name: string;
  pending: boolean;
  transfer?: ResourceTransfer;
}

export interface ResourceTransferController {
  copyResource: (name: string) => void;
  dialogs: ReactNode;
  exportResource: (name: string) => Promise<void>;
  importResource: () => Promise<void>;
}

export function ResourceImportIcon() {
  return <UploadFileOutlinedIcon aria-hidden />;
}

export function useResourceTransferController(options: ResourceTransferControllerOptions): ResourceTransferController {
  const { existingNames, loadResource, onSaved, resourceType, saveResource, showNotice, t } = options;
  const [copyState, setCopyState] = useState<CopyState | null>(null);
  const [importState, setImportState] = useState<ImportState | null>(null);
  const readingClipboard = useRef(false);

  function copyResource(sourceName: string) {
    setCopyState({ error: "", name: "", pending: false, sourceName });
  }

  async function confirmCopy() {
    if (!copyState || copyState.pending) return;
    const issue = copyNameError(copyState.name, copyState.sourceName, existingNames, t);
    if (issue) {
      setCopyState({ ...copyState, error: issue });
      return;
    }
    setCopyState({ ...copyState, error: "", pending: true });
    try {
      const source = requireResource(await loadResource(copyState.sourceName));
      const copied = copyTransferredResource(source, copyState.name);
      await saveResource(copied);
      setCopyState(null);
      showNotice(t(resourceText(resourceType).copied));
      await onSaved(copied);
    } catch (error) {
      setCopyState((current) => current ? { ...current, error: errorMessage(error, t), pending: false } : current);
    }
  }

  async function exportResource(name: string) {
    try {
      const text = serializeResourceTransfer(resourceType, await loadResource(name));
      try {
        if (!navigator.clipboard?.writeText) throw new Error("clipboard unavailable");
        await navigator.clipboard.writeText(text);
      } catch {
        downloadResource(text, resourceTransferFilename(resourceType, name));
      }
      showNotice(t(resourceText(resourceType).exported));
    } catch {
      showNotice(t(resourceText(resourceType).exportFailed), "error");
    }
  }

  async function importResource() {
    if (readingClipboard.current) return;
    readingClipboard.current = true;
    try {
      if (!navigator.clipboard?.readText) {
        setImportState(emptyImportState());
        return;
      }
      let text: string;
      try {
        text = await navigator.clipboard.readText();
      } catch {
        setImportState(emptyImportState());
        return;
      }
      try {
        const transfer = parseResourceTransfer(text, resourceType);
        setImportState(importStateFromTransfer(transfer));
      } catch (error) {
        setImportState({ ...emptyImportState(), error: transferErrorMessage(error, t) });
      }
    } finally {
      readingClipboard.current = false;
    }
  }

  async function selectImportFile(file: File | undefined) {
    if (!file || importState?.pending) return;
    try {
      const transfer = parseResourceTransfer(await file.text(), resourceType);
      setImportState(importStateFromTransfer(transfer));
    } catch (error) {
      setImportState({ ...emptyImportState(), error: transferErrorMessage(error, t) });
    }
  }

  async function confirmImport() {
    if (!importState?.transfer || importState.pending) return;
    const issue = resourceNameError(importState.name, t);
    if (issue) {
      setImportState({ ...importState, error: issue });
      return;
    }
    setImportState({ ...importState, error: "", pending: true });
    const imported = renameTransferredResource(importState.transfer.resource, importState.name);
    try {
      await saveResource(imported);
      setImportState(null);
      showNotice(t(resourceText(resourceType).imported));
      await onSaved(imported);
    } catch (error) {
      setImportState((current) => current ? { ...current, error: errorMessage(error, t), pending: false } : current);
    }
  }

  return {
    copyResource,
    exportResource,
    importResource,
    dialogs: (
      <>
        {copyState ? (
          <CopyResourceDialog
            resourceType={resourceType}
            state={copyState}
            t={t}
            onCancel={() => setCopyState(null)}
            onConfirm={() => { void confirmCopy(); }}
            onNameChange={(name) => setCopyState((current) => current ? { ...current, error: "", name } : current)}
          />
        ) : null}
        {importState ? (
          <ImportResourceDialog
            existing={existingNames.includes(importState.name.trim())}
            resourceType={resourceType}
            state={importState}
            t={t}
            onCancel={() => setImportState(null)}
            onConfirm={() => { void confirmImport(); }}
            onFile={(file) => { void selectImportFile(file); }}
            onNameChange={(name) => setImportState((current) => current ? { ...current, error: "", name } : current)}
          />
        ) : null}
      </>
    ),
  };
}

function CopyResourceDialog({ resourceType, state, t, onCancel, onConfirm, onNameChange }: {
  resourceType: TransferResourceType;
  state: CopyState;
  t: Translator;
  onCancel: () => void;
  onConfirm: () => void;
  onNameChange: (name: string) => void;
}) {
  return (
    <Dialog fullWidth open aria-labelledby="copy-resource-title" maxWidth="sm" onClose={state.pending ? undefined : onCancel}>
      <DialogTitle id="copy-resource-title">{t(resourceText(resourceType).copyTitle)}</DialogTitle>
      <DialogContent>
        <div className="grid gap-4 pt-1">
          <TextField
            fullWidth
            required
            disabled={state.pending}
            error={Boolean(state.error)}
            helperText={state.error}
            label={t("labels.name")}
            value={state.name}
            onChange={(event) => onNameChange(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === "Enter") {
                event.preventDefault();
                onConfirm();
              }
            }}
          />
        </div>
      </DialogContent>
      <DialogActions>
        <Button disabled={state.pending} type="button" onClick={onCancel}>{t("actions.cancel")}</Button>
        <Button disabled={state.pending} startIcon={<ContentCopyOutlinedIcon aria-hidden />} type="button" variant="contained" onClick={onConfirm}>
          {t(state.pending ? "resourceTransfer.copying" : "actions.copy")}
        </Button>
      </DialogActions>
    </Dialog>
  );
}

function ImportResourceDialog({ existing, resourceType, state, t, onCancel, onConfirm, onFile, onNameChange }: {
  existing: boolean;
  resourceType: TransferResourceType;
  state: ImportState;
  t: Translator;
  onCancel: () => void;
  onConfirm: () => void;
  onFile: (file: File | undefined) => void;
  onNameChange: (name: string) => void;
}) {
  return (
    <Dialog fullWidth open aria-labelledby="import-resource-title" maxWidth="sm" onClose={state.pending ? undefined : onCancel}>
      <DialogTitle id="import-resource-title">{t(resourceText(resourceType).importTitle)}</DialogTitle>
      <DialogContent>
        <div className="grid gap-4 pt-1">
          {state.error ? <Alert severity="error">{state.error}</Alert> : null}
          {state.transfer ? (
            <>
              <Typography color="text.secondary" variant="body2">
                {t("resourceTransfer.importType", { type: t(resourceText(resourceType).label) })}
              </Typography>
              <TextField
                fullWidth
                required
                disabled={state.pending}
                label={t("labels.name")}
                value={state.name}
                onChange={(event) => onNameChange(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === "Enter") {
                    event.preventDefault();
                    onConfirm();
                  }
                }}
              />
              {existing ? <Alert severity="warning">{t("resourceTransfer.importOverwrite", { name: state.name.trim() })}</Alert> : null}
            </>
          ) : (
            <>
              <DialogContentText>{t("resourceTransfer.importSelectFile")}</DialogContentText>
              <Button component="label" startIcon={<ResourceImportIcon />} variant="outlined">
                {t("resourceTransfer.selectFile")}
                <input
                  accept=".json,application/json"
                  aria-label={t("resourceTransfer.selectFile")}
                  className="sr-only"
                  type="file"
                  onChange={(event) => {
                    onFile(event.currentTarget.files?.[0]);
                    event.currentTarget.value = "";
                  }}
                />
              </Button>
            </>
          )}
        </div>
      </DialogContent>
      <DialogActions>
        <Button disabled={state.pending} type="button" onClick={onCancel}>{t("actions.cancel")}</Button>
        {state.transfer ? (
          <Button disabled={state.pending} type="button" variant="contained" onClick={onConfirm}>
            {t(state.pending ? "resourceTransfer.importing" : "actions.import")}
          </Button>
        ) : null}
      </DialogActions>
    </Dialog>
  );
}

function importStateFromTransfer(transfer: ResourceTransfer): ImportState {
  return { error: "", name: String(transfer.resource.name), pending: false, transfer };
}

function emptyImportState(): ImportState {
  return { error: "", name: "", pending: false };
}

function requireResource(value: unknown): Record<string, unknown> {
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    throw new Error("resource definition is invalid");
  }
  const resource = value as Record<string, unknown>;
  if (typeof resource.name !== "string" || !resource.name.trim()) {
    throw new Error("resource name is required");
  }
  return resource;
}

function copyNameError(name: string, sourceName: string, existingNames: readonly string[], t: Translator): string {
  const issue = resourceNameError(name, t);
  if (issue) return issue;
  const value = name.trim();
  if (value === sourceName) return t("resourceTransfer.copySameName");
  if (existingNames.includes(value)) return t("resourceTransfer.copyNameExists");
  return "";
}

function resourceNameError(name: string, t: Translator): string {
  switch (resourceNameIssue(name)) {
    case "empty": return t("resourceTransfer.nameRequired");
    case "invalid": return t("resourceTransfer.nameInvalid");
    default: return "";
  }
}

function transferErrorMessage(error: unknown, t: Translator): string {
  if (error instanceof ResourceTransferError) {
    switch (error.code) {
      case "invalid_json": return t("resourceTransfer.invalidJson");
      case "type_mismatch": return t("resourceTransfer.typeMismatch");
      case "missing_name": return t("resourceTransfer.missingName");
      case "invalid_envelope": return t("resourceTransfer.invalidDefinition");
    }
  }
  return errorMessage(error, t);
}

function errorMessage(error: unknown, t: Translator): string {
  return error instanceof Error && error.message ? error.message : t("errors.operationFailedRetry");
}

function downloadResource(text: string, filename: string) {
  const blob = new Blob([text], { type: "application/json" });
  const objectURL = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  try {
    anchor.download = filename;
    anchor.href = objectURL;
    document.body.append(anchor);
    anchor.click();
  } finally {
    anchor.remove();
    URL.revokeObjectURL(objectURL);
  }
}

function resourceText(resourceType: TransferResourceType) {
  return resourceType === "subscription" ? {
    label: "resourceTransfer.subscriptionLabel" as const,
    copyTitle: "resourceTransfer.copySubscription" as const,
    importTitle: "resourceTransfer.importSubscription" as const,
    copied: "resourceTransfer.subscriptionCopied" as const,
    exported: "resourceTransfer.subscriptionExported" as const,
    exportFailed: "resourceTransfer.subscriptionExportFailed" as const,
    imported: "resourceTransfer.subscriptionImported" as const,
  } : {
    label: "resourceTransfer.fileLabel" as const,
    copyTitle: "resourceTransfer.copyFile" as const,
    importTitle: "resourceTransfer.importFile" as const,
    copied: "resourceTransfer.fileCopied" as const,
    exported: "resourceTransfer.fileExported" as const,
    exportFailed: "resourceTransfer.fileExportFailed" as const,
    imported: "resourceTransfer.fileImported" as const,
  };
}
