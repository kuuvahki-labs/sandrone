import { type SyntheticEvent, useState } from "react";
import SaveIcon from "@mui/icons-material/Save";
import Alert from "@mui/material/Alert";

import type { LoadSubscriptionPreview } from "~/features/files/config/components/node-source";
import type { LoadRuleSetCatalog } from "~/features/files/config/components/rule-set-catalog-dialog";
import { FILE_DRIVER_REGISTRY } from "~/features/files/drivers/registry";
import { FileFormFields } from "~/features/files/editor/file-form";
import { useI18n } from "~/shared/i18n/context";
import type { ResourceOption } from "~/shared/resources/types";
import type { FileCreateSource } from "~/shared/routing/paths";
import { DiscardChangesDialog } from "~/shared/ui/dialogs";
import { PageHeader } from "~/shared/ui/page";

export interface FileNewPageProps {
  loadSubscriptionPreview?: LoadSubscriptionPreview;
  loadRuleSetCatalog?: LoadRuleSetCatalog;
  source: FileCreateSource;
  onBack: () => void;
  onSave: (kind: string, form: FormData) => void | Promise<void>;
  scriptFiles?: ResourceOption[];
  subscriptions?: ResourceOption[];
}

export function FileNewPage({ loadRuleSetCatalog, loadSubscriptionPreview, source, onBack, onSave, scriptFiles, subscriptions }: FileNewPageProps) {
  const { t } = useI18n();
  const [dirty, setDirty] = useState(false);
  const [confirmLeave, setConfirmLeave] = useState(false);
  const [error, setError] = useState("");
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
    try {
      await onSave(driver.kind, new FormData(event.currentTarget));
    } catch (submitError) {
      setError(submitError instanceof Error ? submitError.message : t("errors.fileSaveFailed"));
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
          scriptFiles={scriptFiles}
          sourceDefault={createPreset.sourceType === "remote" ? { type: "remote", remote: {} } : { type: "inline", content: "" }}
          sourceEditorKey={source}
          subscriptions={subscriptions}
        />
        {error ? <Alert severity="error">{error}</Alert> : null}
      </form>
      {confirmLeave ? <DiscardChangesDialog onCancel={() => setConfirmLeave(false)} onConfirm={onBack} /> : null}
    </section>
  );
}
