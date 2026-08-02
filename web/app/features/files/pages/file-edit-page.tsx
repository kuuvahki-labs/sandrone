import { useCallback, useRef, useState } from "react";
import SaveIcon from "@mui/icons-material/Save";
import ShareOutlinedIcon from "@mui/icons-material/ShareOutlined";
import VisibilityIcon from "@mui/icons-material/Visibility";
import Alert from "@mui/material/Alert";

import type { LoadSubscriptionPreview } from "~/features/files/config/components/node-source";
import type { LoadRuleSetCatalog } from "~/features/files/config/components/rule-set-catalog-dialog";
import { fileDriver } from "~/features/files/drivers/registry";
import { FileFormFields } from "~/features/files/editor/file-form";
import type { FileDetail, FileItem } from "~/features/files/model/types";
import { useI18n } from "~/shared/i18n/context";
import {
  changeEditSession,
  createEditSession,
  finishEditSessionSave,
  isEditSessionDirty,
  isEditSessionSaving,
  startEditSessionSave,
} from "~/shared/resources/edit-session";
import {
  persistedResourceActionBlocker,
  persistedResourceActionDisabledReason,
} from "~/shared/resources/persisted-resource-actions";
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
  onShare: () => void;
  scriptFiles?: ResourceOption[];
  subscriptions?: ResourceOption[];
}

export function FileEditPage({ detail, detailPending = false, item, loadRuleSetCatalog, loadSubscriptionPreview, onBack, onPreview, onSave, onShare, scriptFiles, subscriptions }: FileEditPageProps) {
  const { t } = useI18n();
  const [editSession, setEditSession] = useState(createEditSession);
  const [confirmLeave, setConfirmLeave] = useState(false);
  const [valid, setValid] = useState(true);
  const editSessionRef = useRef(editSession);
  const savingRef = useRef(false);
  const dirty = isEditSessionDirty(editSession);
  const saving = isEditSessionSaving(editSession);
  const actionBlocker = persistedResourceActionBlocker({
    dirty,
    loading: detailPending,
    saving,
  });
  const previewDisabledReason = persistedResourceActionDisabledReason("preview", actionBlocker, t);
  const shareDisabledReason = persistedResourceActionDisabledReason("share", actionBlocker, t);
  const meta = detail?.meta ?? (item.description ? { description: item.description } : {});
  const driver = fileDriver(detail?.kind ?? item.kind);
  const markDirty = useCallback(() => {
    const changed = changeEditSession(editSessionRef.current);
    editSessionRef.current = changed;
    setEditSession(changed);
  }, []);

  function requestBack() {
    if (dirty) {
      setConfirmLeave(true);
      return;
    }
    onBack();
  }

  async function save(form: FormData) {
    if (savingRef.current) return;
    const started = startEditSessionSave(editSessionRef.current);
    savingRef.current = true;
    editSessionRef.current = started;
    setEditSession(started);
    let persisted = false;
    try {
      await onSave(form);
      persisted = true;
    } finally {
      const finished = finishEditSessionSave(editSessionRef.current, persisted);
      editSessionRef.current = finished;
      savingRef.current = false;
      setEditSession(finished);
    }
  }

  if (!driver) {
    return (
      <section className="grid gap-6">
        <PageHeader
          backAction={{ label: t("actions.back"), onSelect: onBack }}
          label=""
          secondaryActions={[
            { accessibleLabel: t("files.actions.preview"), icon: <VisibilityIcon aria-hidden fontSize="small" />, label: t("common.preview"), onSelect: onPreview },
            { accessibleLabel: t("files.actions.share"), icon: <ShareOutlinedIcon aria-hidden fontSize="small" />, label: t("actions.share"), onSelect: onShare },
          ]}
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
      <form className="grid gap-6" onChange={markDirty} onSubmit={(event) => { event.preventDefault(); void save(new FormData(event.currentTarget)); }}>
        <PageHeader
          backAction={{ label: t("actions.back"), onSelect: requestBack }}
          label=""
          primaryAction={{ accessibleLabel: t("files.actions.save"), disabled: detailPending || saving || !valid, icon: <SaveIcon aria-hidden fontSize="small" />, label: t("actions.save"), type: "submit", variant: "contained" }}
          secondaryActions={[
            { accessibleLabel: t("files.actions.preview"), disabled: Boolean(previewDisabledReason), disabledReason: previewDisabledReason, icon: <VisibilityIcon aria-hidden fontSize="small" />, label: t("common.preview"), onSelect: onPreview },
            { accessibleLabel: t("files.actions.share"), disabled: Boolean(shareDisabledReason), disabledReason: shareDisabledReason, icon: <ShareOutlinedIcon aria-hidden fontSize="small" />, label: t("actions.share"), onSelect: onShare },
          ]}
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
          onDirty={markDirty}
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
