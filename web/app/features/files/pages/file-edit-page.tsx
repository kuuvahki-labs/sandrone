import { useState } from "react";
import SaveIcon from "@mui/icons-material/Save";
import VisibilityIcon from "@mui/icons-material/Visibility";
import Alert from "@mui/material/Alert";

import type { LoadSubscriptionPreview } from "~/features/files/config/components/node-source";
import type { LoadRuleSetCatalog } from "~/features/files/config/components/rule-set-catalog-dialog";
import { fileDriver } from "~/features/files/drivers/registry";
import { FileFormFields } from "~/features/files/editor/file-form";
import type { FileDetail, FileItem } from "~/features/files/model/types";
import { useI18n } from "~/shared/i18n/context";
import type { ResourceOption } from "~/shared/resources/types";
import { CodeBlock } from "~/shared/ui/code-editor";
import { DiscardChangesDialog } from "~/shared/ui/dialogs";
import { PageHeader } from "~/shared/ui/page";

export interface FileEditPageProps {
  detail?: FileDetail | null;
  detailPending?: boolean;
  item: FileItem;
  loadSubscriptionPreview?: LoadSubscriptionPreview;
  loadRuleSetCatalog?: LoadRuleSetCatalog;
  onBack: () => void;
  onPreview: () => void;
  onSave: (form: FormData) => void | Promise<void>;
  scriptFiles?: ResourceOption[];
  subscriptions?: ResourceOption[];
}

export function FileEditPage({ detail, detailPending = false, item, loadRuleSetCatalog, loadSubscriptionPreview, onBack, onPreview, onSave, scriptFiles, subscriptions }: FileEditPageProps) {
  const { t } = useI18n();
  const [dirty, setDirty] = useState(false);
  const [confirmLeave, setConfirmLeave] = useState(false);
  const [valid, setValid] = useState(true);
  const meta = detail?.meta ?? (item.description ? { description: item.description } : {});
  const driver = fileDriver(detail?.kind ?? item.kind);

  function requestBack() {
    if (dirty) {
      setConfirmLeave(true);
      return;
    }
    onBack();
  }

  if (!driver) {
    return (
      <section className="grid gap-6">
        <PageHeader
          backAction={{ label: t("actions.back"), onSelect: onBack }}
          label=""
          secondaryActions={[{ accessibleLabel: t("files.actions.preview"), icon: <VisibilityIcon aria-hidden fontSize="small" />, label: t("common.preview"), onSelect: onPreview }]}
          sticky
          title={t("files.edit.title")}
        />
        <Alert severity="info">{t("files.edit.unknownKindReadonly")}</Alert>
        <CodeBlock
          label={t("files.edit.rawDefinition")}
          language="json"
          value={JSON.stringify(detail?.rawSpec ?? {}, null, 2)}
        />
      </section>
    );
  }

  return (
    <section className="grid gap-6">
      <form className="grid gap-6" onChange={() => setDirty(true)} onSubmit={(event) => { event.preventDefault(); void onSave(new FormData(event.currentTarget)); }}>
        <PageHeader
          backAction={{ label: t("actions.back"), onSelect: requestBack }}
          label=""
          primaryAction={{ accessibleLabel: t("files.actions.save"), disabled: detailPending || !valid, icon: <SaveIcon aria-hidden fontSize="small" />, label: t("actions.save"), type: "submit", variant: "contained" }}
          secondaryActions={[{ accessibleLabel: t("files.actions.preview"), disabled: detailPending, icon: <VisibilityIcon aria-hidden fontSize="small" />, label: t("common.preview"), onSelect: onPreview }]}
          sticky
          title={t("files.edit.title")}
        />

        <FileFormFields
          defaultName={detail?.name || item.name}
          configDefault={detail?.config}
          description={meta.description ?? item.description ?? ""}
          displayName={detail?.displayName ?? item.displayName ?? ""}
          driver={driver}
          loadSubscriptionPreview={loadSubscriptionPreview}
          loadRuleSetCatalog={loadRuleSetCatalog}
          mode="edit"
          onDirty={() => setDirty(true)}
          onValidityChange={setValid}
          processorsDefault={detail?.processors}
          renderCacheTTLSeconds={detail?.renderCacheTTLSeconds}
          scriptFiles={scriptFiles}
          sourceDefault={detail?.source}
          subscriptions={subscriptions}
        />
      </form>
      {confirmLeave ? <DiscardChangesDialog onCancel={() => setConfirmLeave(false)} onConfirm={onBack} /> : null}
    </section>
  );
}
