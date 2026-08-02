import type { TranslationKey, Translator } from "~/shared/i18n/context";

export type PersistedResourceAction = "preview" | "share";
export type PersistedResourceActionBlocker = "loading" | "saving" | "dirty";

const reasonKeys: Record<
  PersistedResourceAction,
  Record<PersistedResourceActionBlocker, TranslationKey>
> = {
  preview: {
    dirty: "resourceActions.preview.dirty",
    loading: "resourceActions.preview.loading",
    saving: "resourceActions.preview.saving",
  },
  share: {
    dirty: "resourceActions.share.dirty",
    loading: "resourceActions.share.loading",
    saving: "resourceActions.share.saving",
  },
};

export function persistedResourceActionBlocker({
  dirty,
  loading,
  saving,
}: {
  dirty: boolean;
  loading: boolean;
  saving: boolean;
}): PersistedResourceActionBlocker | null {
  if (loading) return "loading";
  if (saving) return "saving";
  if (dirty) return "dirty";
  return null;
}

export function persistedResourceActionDisabledReason(
  action: PersistedResourceAction,
  blocker: PersistedResourceActionBlocker | null,
  t: Translator,
): string | undefined {
  return blocker ? t(reasonKeys[action][blocker]) : undefined;
}
