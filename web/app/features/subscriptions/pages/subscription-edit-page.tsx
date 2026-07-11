import { useEffect, useState } from "react";
import SaveIcon from "@mui/icons-material/Save";
import VisibilityOutlinedIcon from "@mui/icons-material/VisibilityOutlined";

import { type SubscriptionCopyTarget, SubscriptionFormFields } from "~/features/subscriptions/components/subscription-form";
import type { SubscriptionDefinition, SubscriptionItem } from "~/features/subscriptions/model/types";
import { useI18n } from "~/shared/i18n/context";
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
  onSave: (form: FormData) => void | Promise<void>;
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
}: SubscriptionEditPageProps) {
  const { t } = useI18n();
  const [dirty, setDirty] = useState(false);
  const [confirmLeave, setConfirmLeave] = useState(false);
  const [selectedType, setSelectedType] = useState<SubscriptionCreateType>(item.kind);

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

  return (
    <section className="grid gap-6">
      <form className="grid gap-6" onChange={() => setDirty(true)} onSubmit={(event) => { event.preventDefault(); void onSave(new FormData(event.currentTarget)); }}>
        <PageHeader
          backAction={{ label: t("actions.back"), onSelect: requestBack }}
          label=""
          primaryAction={{ accessibleLabel: t("subscriptions.save"), disabled: definitionPending, icon: <SaveIcon aria-hidden fontSize="small" />, label: t("actions.save"), type: "submit", variant: "contained" }}
          secondaryActions={onPreview ? [{ accessibleLabel: t("subscriptions.actions.preview"), disabled: definitionPending, icon: <VisibilityOutlinedIcon aria-hidden fontSize="small" />, label: t("common.preview"), onSelect: onPreview }] : []}
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
            onTypeChange={setSelectedType}
          />
        </div>
      </form>
      {confirmLeave ? <DiscardChangesDialog onCancel={() => setConfirmLeave(false)} onConfirm={onBack} /> : null}
    </section>
  );
}
