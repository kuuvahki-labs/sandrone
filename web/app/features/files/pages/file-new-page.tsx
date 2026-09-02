import { type SyntheticEvent, useState } from "react";
import SaveIcon from "@mui/icons-material/Save";
import Alert from "@mui/material/Alert";

import type { LoadSubscriptionPreview } from "~/features/files/config/components/node-source";
import type { LoadRuleSetCatalog } from "~/features/files/config/components/rule-set-catalog-dialog";
import { FILE_DRIVER_REGISTRY } from "~/features/files/drivers/registry";
import { FileFormFields } from "~/features/files/editor/file-form";
import type { FileItem } from "~/features/files/model/types";
import { useI18n } from "~/shared/i18n/context";
import type { RemoteInputDefaults, ResourceOption } from "~/shared/resources/types";
import type { FileCreateSource } from "~/shared/routing/paths";
import { DiscardChangesDialog, OverwriteResourceDialog } from "~/shared/ui/dialogs";
import { PageHeader } from "~/shared/ui/page";

export interface FileNewPageProps {
  existingFiles?: FileItem[];
  loadSubscriptionPreview?: LoadSubscriptionPreview;
  loadRuleSetCatalog?: LoadRuleSetCatalog;
  source: FileCreateSource;
  onBack: () => void;
  onSave: (kind: string, form: FormData, existing?: FileItem) => void | Promise<void>;
  remoteDefaults?: RemoteInputDefaults;
  scriptFiles?: ResourceOption[];
  scriptTimeoutMS?: number;
  subscriptions?: ResourceOption[];
}

interface PendingFileOverwrite {
  existing: FileItem;
  form: FormData;
  kind: string;
}

export function FileNewPage({ existingFiles = [], loadRuleSetCatalog, loadSubscriptionPreview, source, onBack, onSave, remoteDefaults, scriptFiles, scriptTimeoutMS, subscriptions }: FileNewPageProps) {
  const { t } = useI18n();
  const [dirty, setDirty] = useState(false);
  const [confirmLeave, setConfirmLeave] = useState(false);
  const [error, setError] = useState("");
  const [overwritePending, setOverwritePending] = useState(false);
  const [pendingOverwrite, setPendingOverwrite] = useState<PendingFileOverwrite | null>(null);
  const [valid, setValid] = useState(true);
  const createPreset = FILE_DRIVER_REGISTRY.resolveCreatePreset(source) ?? FILE_DRIVER_REGISTRY.resolveCreatePreset("local")!;
  const driver = FILE_DRIVER_REGISTRY.get(createPreset.kind)!;

  function requestBack() {
    if (dirty) {
      setConfirmLeave(true);
      return;
    }
    onBack();
  }

  async function handleSubmit(event: SyntheticEvent<HTMLFormElement, SubmitEvent>) {
    event.preventDefault();
    setError("");
    const form = new FormData(event.currentTarget);
    const name = String(form.get("name") ?? "").trim();
    const existing = existingFiles.find((item) => item.name === name);
    if (existing) {
      setPendingOverwrite({ existing, form, kind: driver.kind });
      return;
    }
    try {
      await onSave(driver.kind, form);
    } catch (submitError) {
      setError(submitError instanceof Error ? submitError.message : t("errors.fileSaveFailed"));
    }
  }

  async function confirmOverwrite() {
    if (!pendingOverwrite) return;
    setError("");
    setOverwritePending(true);
    try {
      await onSave(pendingOverwrite.kind, pendingOverwrite.form, pendingOverwrite.existing);
      setPendingOverwrite(null);
    } catch (submitError) {
      setPendingOverwrite(null);
      setError(submitError instanceof Error ? submitError.message : t("errors.fileSaveFailed"));
    } finally {
      setOverwritePending(false);
    }
  }

  return (
    <section className="grid gap-6">
      <form className="grid gap-6" onChange={() => setDirty(true)} onSubmit={(event) => { void handleSubmit(event); }}>
        <PageHeader
          backAction={{ label: t("actions.back"), onSelect: requestBack }}
          label=""
          primaryAction={{ accessibleLabel: t("files.actions.save"), disabled: !valid, icon: <SaveIcon aria-hidden fontSize="small" />, label: t("actions.save"), type: "submit", variant: "contained" }}
          sticky
          title={t("files.new.title")}
        />

        <FileFormFields
          defaultName={createPreset.initialName}
          driver={driver}
          key={`${driver.kind}:${createPreset.source}`}
          loadSubscriptionPreview={loadSubscriptionPreview}
          loadRuleSetCatalog={loadRuleSetCatalog}
          mode="create"
          onDirty={() => setDirty(true)}
          onValidityChange={setValid}
          remoteDefaults={remoteDefaults}
          scriptFiles={scriptFiles}
          scriptTimeoutMS={scriptTimeoutMS}
          sourceDefault={createPreset.sourceType === "remote" ? { type: "remote", remote: {} } : { type: "inline", content: "" }}
          sourceEditorKey={source}
          subscriptions={subscriptions}
        />
        {error ? <Alert severity="error">{error}</Alert> : null}
      </form>
      {confirmLeave ? <DiscardChangesDialog onCancel={() => setConfirmLeave(false)} onConfirm={onBack} /> : null}
      {pendingOverwrite ? <OverwriteResourceDialog name={pendingOverwrite.existing.name} pending={overwritePending} resource="file" onCancel={() => setPendingOverwrite(null)} onConfirm={() => { void confirmOverwrite(); }} /> : null}
    </section>
  );
}
