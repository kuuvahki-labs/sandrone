import { useCallback, useEffect, useRef, useState } from "react";
import SaveIcon from "@mui/icons-material/Save";
import ShareOutlinedIcon from "@mui/icons-material/ShareOutlined";
import VisibilityOutlinedIcon from "@mui/icons-material/VisibilityOutlined";

import { type SubscriptionCopyTarget, SubscriptionFormFields } from "~/features/subscriptions/components/subscription-form";
import type { SubscriptionDefinition, SubscriptionItem } from "~/features/subscriptions/model/types";
import { useI18n } from "~/shared/i18n/context";
import {
  changeEditSession,
  createEditSession,
  finishEditSessionSave,
  isEditSessionDirty,
  isEditSessionSaving,
  startEditSessionSave,
} from "~/shared/resources/edit-session";
import type { ResourceOption } from "~/shared/resources/types";
import type { SubscriptionCreateType } from "~/shared/routing/paths";
import { DiscardChangesDialog } from "~/shared/ui/dialogs";
import { PageHeader } from "~/shared/ui/page";

export interface SubscriptionEditPageProps {
  item: SubscriptionItem;
  definition?: SubscriptionDefinition | null;
  definitionPending?: boolean;
  scriptFiles?: ResourceOption[];
  sources: SubscriptionItem[];
  onBack: () => void;
  onCopySource?: (value: string, target: SubscriptionCopyTarget) => void | Promise<void>;
  onPreview?: () => void;
  onSave: (form: FormData) => boolean | Promise<boolean>;
  onShare?: () => void;
}

export function SubscriptionEditPage({
  item,
  definition,
  definitionPending = false,
  scriptFiles,
  sources,
  onBack,
  onCopySource,
  onPreview,
  onSave,
  onShare,
}: SubscriptionEditPageProps) {
  const { t } = useI18n();
  const [editSession, setEditSession] = useState(createEditSession);
  const [confirmLeave, setConfirmLeave] = useState(false);
  const [selectedType, setSelectedType] = useState<SubscriptionCreateType>(item.kind);
  const editSessionRef = useRef(editSession);
  const savingRef = useRef(false);
  const dirty = isEditSessionDirty(editSession);
  const saving = isEditSessionSaving(editSession);
  const markDirty = useCallback(() => {
    const changed = changeEditSession(editSessionRef.current);
    editSessionRef.current = changed;
    setEditSession(changed);
  }, []);

  useEffect(() => {
    setSelectedType(item.kind);
  }, [item.kind, item.name]);

  function requestBack() {
    if (dirty) {
      setConfirmLeave(true);
      return;
    }
    onBack();
  }

  function selectType(type: SubscriptionCreateType) {
    if (type === selectedType) return;
    setSelectedType(type);
    markDirty();
  }

  async function save(form: FormData) {
    if (savingRef.current) return;
    const started = startEditSessionSave(editSessionRef.current);
    savingRef.current = true;
    editSessionRef.current = started;
    setEditSession(started);
    let persisted = false;
    try {
      persisted = await onSave(form);
    } finally {
      const finished = finishEditSessionSave(editSessionRef.current, persisted);
      editSessionRef.current = finished;
      savingRef.current = false;
      setEditSession(finished);
    }
  }

  return (
    <section className="grid gap-6">
      <form className="grid gap-6" onChange={markDirty} onSubmit={(event) => { event.preventDefault(); void save(new FormData(event.currentTarget)); }}>
        <PageHeader
          backAction={{ label: t("actions.back"), onSelect: requestBack }}
          label=""
          primaryAction={{ accessibleLabel: t("subscriptions.save"), disabled: definitionPending || saving, icon: <SaveIcon aria-hidden fontSize="small" />, label: t("actions.save"), type: "submit", variant: "contained" }}
          secondaryActions={[
            ...(onPreview ? [{ accessibleLabel: t("subscriptions.actions.preview"), disabled: definitionPending, icon: <VisibilityOutlinedIcon aria-hidden fontSize="small" />, label: t("common.preview"), onSelect: onPreview }] : []),
            ...(onShare ? [{ accessibleLabel: t("subscriptions.actions.share"), disabled: definitionPending || dirty || saving, icon: <ShareOutlinedIcon aria-hidden fontSize="small" />, label: t("actions.share"), onSelect: onShare }] : []),
          ]}
          sticky
          title={t("subscriptions.edit.title")}
        />

        <div className="grid gap-4">
          <SubscriptionFormFields
            definition={definition}
            item={item}
            mode="edit"
            scriptFiles={scriptFiles}
            sources={sources}
            type={selectedType}
            onCopySource={onCopySource}
            onDirty={markDirty}
            onTypeChange={selectType}
          />
        </div>
      </form>
      {confirmLeave ? <DiscardChangesDialog onCancel={() => setConfirmLeave(false)} onConfirm={onBack} /> : null}
    </section>
  );
}
