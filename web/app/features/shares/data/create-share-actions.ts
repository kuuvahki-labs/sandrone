import { shareFromCreateResponse } from "~/features/shares/model/codec";
import { type ShareCopyFormat, shareUrlWithFormat } from "~/features/shares/model/share-formats";
import type { ShareItem } from "~/features/shares/model/types";
import type { ApiClient, ShareCreateRequest } from "~/shared/api/client";
import { defaultTranslator, type Translator } from "~/shared/i18n/context";

export type CopyShareResult =
  | { copied: true }
  | { copied: false; url: string };

export function createShareActions({
  client,
  publicBaseUrl,
  showNotice,
  t = defaultTranslator(),
}: {
  client: ApiClient;
  publicBaseUrl: string;
  showNotice: (message: string, severity?: "success" | "error" | "warning") => void;
  t?: Translator;
}) {
  async function createShare(form: FormData) {
    const targetKind = String(form.get("target_kind") ?? "file") === "subscription" ? "subscription" : "file";
    const targetName = String(form.get("target") ?? "").trim();
    if (!targetName) {
      showNotice(targetKind === "subscription" ? t("messages.needSubscriptionBeforeShare") : t("messages.needFileBeforeShare"), "warning");
      return null;
    }
    const payload: ShareCreateRequest = {
      name: String(form.get("name") ?? "").trim() || targetName,
      target_kind: targetKind,
      target_name: targetName,
      meta: { ui: "web" },
    };
    if (targetKind === "subscription") {
      payload.target_format = String(form.get("target_format") ?? "").trim() || "base64";
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

    const response = await client.createShare(payload);
    return shareFromCreateResponse(response, publicBaseUrl);
  }

  async function copyShareUrl(publicUrl: string): Promise<CopyShareResult> {
    if (!navigator.clipboard) {
      showNotice(t("shares.messages.copyUnavailable"), "warning");
      return { copied: false, url: publicUrl };
    }
    try {
      await navigator.clipboard.writeText(publicUrl);
    } catch {
      showNotice(t("shares.messages.copyUnavailable"), "warning");
      return { copied: false, url: publicUrl };
    }
    showNotice(t("messages.linkCopied"));
    return { copied: true };
  }

  async function copyShare(item: ShareItem, format?: ShareCopyFormat): Promise<CopyShareResult> {
    const publicUrl = format
      ? shareUrlWithFormat(item.publicUrl, format, item.formatFilenames?.[format])
      : item.publicUrl;
    return copyShareUrl(publicUrl);
  }

  return {
    copyShare,
    copyShareUrl,
    createShare,
  };
}

function isoDateTime(form: FormData, name: string): string | undefined {
  const value = String(form.get(name) ?? "").trim();
  if (!value) return undefined;
  const timestamp = Date.parse(value);
  return Number.isFinite(timestamp) ? new Date(timestamp).toISOString() : undefined;
}
