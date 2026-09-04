import { shareFromCreateResponse } from "~/features/shares/model/codec";
import { type ShareCopyFormat, shareUrlWithFormat } from "~/features/shares/model/share-formats";
import type { ShareItem } from "~/features/shares/model/types";
import type { ApiClient, ShareCreateRequest } from "~/shared/api/client";
import { defaultTranslator, type Translator } from "~/shared/i18n/context";
import { copyText } from "~/shared/ui/text-transfer";

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

    const response = await client.createShare(payload);
    return shareFromCreateResponse(response, publicBaseUrl);
  }

  async function copyShareUrl(publicUrl: string): Promise<CopyShareResult> {
    if (!(await copyText(publicUrl))) {
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
