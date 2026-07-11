import { useState } from "react";

import type { ApiClient } from "~/shared/api/client";
import { defaultTranslator, type Translator } from "~/shared/i18n/context";

import type { DeleteTarget, ShowNotice } from "./types";

export function useResourceDelete({
  client,
  showNotice,
  t = defaultTranslator(),
}: {
  client: ApiClient;
  showNotice: ShowNotice;
  t?: Translator;
}) {
  const [deleteTarget, setDeleteTarget] = useState<DeleteTarget | null>(null);

  async function confirmDelete() {
    if (!deleteTarget) return;
    await client.deleteResource(deleteTarget.kind, deleteTarget.name);
    setDeleteTarget(null);
    await deleteTarget.onDeleted?.();
    showNotice(t("messages.deleted", { label: deleteTarget.label }));
  }

  return {
    cancelDelete: () => setDeleteTarget(null),
    confirmDelete,
    deleteTarget,
    requestDelete: setDeleteTarget,
  };
}
