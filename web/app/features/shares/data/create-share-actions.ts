import { type ShareCopyFormat, shareUrlWithFormat } from "~/features/shares/model/share-formats";
import type { ShareItem } from "~/features/shares/model/types";
import type { ApiClient, ShareCreateRequest } from "~/shared/api/client";
import { defaultTranslator, type Translator } from "~/shared/i18n/context";

export function createShareActions({
  client,
  closeSheet,
  onShareCreated,
  showNotice,
  t = defaultTranslator(),
}: {
  client: ApiClient;
  closeSheet: () => void;
  onShareCreated?: () => Promise<void>;
  showNotice: (message: string, severity?: "success" | "error" | "warning") => void;
  t?: Translator;
}) {
  async function createShare(form: FormData) {
    const targetKind = String(form.get("target_kind") ?? "file") === "subscription" ? "subscription" : "file";
    const targetName = String(form.get("target") ?? "").trim();
    if (!targetName) {
      showNotice(targetKind === "subscription" ? t("messages.needSubscriptionBeforeShare") : t("messages.needFileBeforeShare"), "warning");
      return;
    }
    const payload: ShareCreateRequest = {
      name: String(form.get("name") ?? "").trim() || targetName,
      target_kind: targetKind,
      target_name: targetName,
      meta: { ui: "web" },
    };
    if (targetKind === "subscription") {
      payload.target_format = String(form.get("target_format") ?? "").trim() || "uri-list";
    } else {
      payload.content_type = "application/octet-stream";
    }
    const validFrom = isoDateTime(form, "valid_from");
    const validUntil = isoDateTime(form, "valid_until");
    if (validFrom) payload.valid_from = validFrom;
    if (validUntil) payload.valid_until = validUntil;
	const ageRecipient = String(form.get("age_recipient") ?? "").trim();
	if (ageRecipient) payload.age_recipient = ageRecipient;
	const maxUses = Number.parseInt(String(form.get("max_uses") ?? ""), 10);
	if (Number.isFinite(maxUses) && maxUses > 0) payload.max_uses = maxUses;

    await client.createShare(payload);
    await onShareCreated?.();
    closeSheet();
    showNotice(t("messages.shareCreated"));
  }

  async function copyShare(item: ShareItem, format?: ShareCopyFormat) {
    if (navigator.clipboard) {
      await navigator.clipboard.writeText(format ? shareUrlWithFormat(item.publicUrl, format) : item.publicUrl);
    }
    showNotice(t("messages.linkCopied"));
  }

  return {
    copyShare,
    createShare,
  };
}

function isoDateTime(form: FormData, name: string): string | undefined {
  const value = String(form.get(name) ?? "").trim();
  if (!value) return undefined;
  const timestamp = Date.parse(value);
  return Number.isFinite(timestamp) ? new Date(timestamp).toISOString() : undefined;
}
