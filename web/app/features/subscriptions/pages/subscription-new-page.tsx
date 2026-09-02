import { type SyntheticEvent, useState } from "react";
import SaveIcon from "@mui/icons-material/Save";
import Alert from "@mui/material/Alert";

import { type SubscriptionCopyTarget, SubscriptionFormFields } from "~/features/subscriptions/components/subscription-form";
import { subscriptionCreateName } from "~/features/subscriptions/data/create-subscription-actions";
import type { SubscriptionItem } from "~/features/subscriptions/model/types";
import type { ProbeDefaultsInput } from "~/shared/api/client";
import { useI18n } from "~/shared/i18n/context";
import type { RemoteInputDefaults, ResourceOption } from "~/shared/resources/types";
import type { SubscriptionCreateType } from "~/shared/routing/paths";
import { DiscardChangesDialog, OverwriteResourceDialog } from "~/shared/ui/dialogs";
import { PageHeader } from "~/shared/ui/page";

export interface SubscriptionNewPageProps {
  sources: SubscriptionItem[];
  type: SubscriptionCreateType;
  onBack: () => void;
  onCopySource?: (value: string, target: SubscriptionCopyTarget) => void | Promise<void>;
  onSave: (form: FormData, existing?: SubscriptionItem) => void | Promise<void>;
  onTypeChange: (type: SubscriptionCreateType) => void;
  probeCacheTTLSeconds: number;
  probeDefaults: ProbeDefaultsInput;
  remoteDefaults?: RemoteInputDefaults;
  scriptFiles?: ResourceOption[];
  scriptTimeoutMS?: number;
}

interface PendingSubscriptionOverwrite {
  existing: SubscriptionItem;
  form: FormData;
}

export function SubscriptionNewPage({ sources, type, onBack, onCopySource, onSave, onTypeChange, probeCacheTTLSeconds, probeDefaults, remoteDefaults, scriptFiles, scriptTimeoutMS }: SubscriptionNewPageProps) {
  const { t } = useI18n();
  const [dirty, setDirty] = useState(false);
  const [confirmLeave, setConfirmLeave] = useState(false);
  const [error, setError] = useState("");
  const [overwritePending, setOverwritePending] = useState(false);
  const [pendingOverwrite, setPendingOverwrite] = useState<PendingSubscriptionOverwrite | null>(null);

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
    const name = subscriptionCreateName(form);
    const existing = sources.find((item) => item.name === name);
    if (existing) {
      setPendingOverwrite({ existing, form });
      return;
    }
    try {
      await onSave(form);
    } catch (submitError) {
      setError(submitError instanceof Error ? submitError.message : t("subscriptions.saveFailed"));
    }
  }

  async function confirmOverwrite() {
    if (!pendingOverwrite) return;
    setError("");
    setOverwritePending(true);
    try {
      await onSave(pendingOverwrite.form, pendingOverwrite.existing);
      setPendingOverwrite(null);
    } catch (submitError) {
      setPendingOverwrite(null);
      setError(submitError instanceof Error ? submitError.message : t("subscriptions.saveFailed"));
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
          primaryAction={{ accessibleLabel: t("subscriptions.save"), icon: <SaveIcon aria-hidden fontSize="small" />, label: t("actions.save"), type: "submit", variant: "contained" }}
          sticky
          title={t("subscriptions.create")}
        />

        <SubscriptionFormFields mode="create" probeCacheTTLSeconds={probeCacheTTLSeconds} probeDefaults={probeDefaults} remoteDefaults={remoteDefaults} scriptFiles={scriptFiles} scriptTimeoutMS={scriptTimeoutMS} sources={sources} type={type} onCopySource={onCopySource} onTypeChange={onTypeChange} />
        {error ? <Alert severity="error">{error}</Alert> : null}
      </form>
      {confirmLeave ? <DiscardChangesDialog onCancel={() => setConfirmLeave(false)} onConfirm={onBack} /> : null}
      {pendingOverwrite ? <OverwriteResourceDialog name={pendingOverwrite.existing.name} pending={overwritePending} resource="subscription" onCancel={() => setPendingOverwrite(null)} onConfirm={() => { void confirmOverwrite(); }} /> : null}
    </section>
  );
}
