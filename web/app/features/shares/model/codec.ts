import { arrayField, asRecord, stringField } from "~/shared/resources/model-fields";

import type { ShareItem } from "./types";

export function sharesFromShareList(list: unknown, publicBaseUrl = ""): ShareItem[] {
  const base = publicBaseUrl.trim().replace(/\/+$/, "");
  return arrayField(asRecord(list).shares).map((value) => {
    const item = asRecord(value);
    const id = stringField(item.id);
    const targetName = stringField(item.target_name);
    const targetKind = shareTargetKindFromAPI(item.target_kind);
    const targetFormat = stringField(item.target_format) || undefined;
    const validFrom = stringField(item.valid_from) || undefined;
    const validUntil = stringField(item.valid_until) || undefined;
    const ageRecipient = stringField(item.age_recipient) || undefined;
    const maxUses = positiveNumberField(item.max_uses) || undefined;
    const useCount = nonNegativeNumberField(item.use_count);
    const rawUrl = `${base}/s/${id}`.replace(/^\/s/, "/s");
    const publicUrl = targetKind === "subscription" && targetFormat
      ? `${rawUrl}?format=${encodeURIComponent(targetFormat)}`
      : rawUrl;
    return {
      id,
      title: stringField(item.name) || id,
      targetKind,
      targetName,
      targetFormat,
      validFrom,
      validUntil,
      ageRecipient,
      maxUses,
      useCount,
      status: shareStatus(validFrom, validUntil, maxUses, useCount),
      publicUrl,
    };
  }).filter((item) => item.id && item.targetKind !== undefined);
}

function shareTargetKindFromAPI(value: unknown): ShareItem["targetKind"] | undefined {
  const kind = stringField(value);
  if (kind === "file" || kind === "subscription") {
    return kind;
  }
  return undefined;
}

function shareStatus(
  validFrom?: string,
  validUntil?: string,
  maxUses?: number,
  useCount = 0,
): ShareItem["status"] {
  const now = Date.now();
  const from = validFrom ? Date.parse(validFrom) : Number.NaN;
  const until = validUntil ? Date.parse(validUntil) : Number.NaN;
  if (Number.isFinite(from) && now < from) return "upcoming";
  if (Number.isFinite(until) && now >= until) return "expired";
  if (maxUses && useCount >= maxUses) return "exhausted";
  return "valid";
}

function positiveNumberField(value: unknown): number {
  return typeof value === "number" && Number.isFinite(value) && value > 0 ? value : 0;
}

function nonNegativeNumberField(value: unknown): number {
  return typeof value === "number" && Number.isFinite(value) && value >= 0 ? value : 0;
}
