import type { TranslationKey } from "~/shared/i18n/context";

export type ShareCopyFormat = "uri-list" | "mihomo-proxies" | "sing-box-outbounds" | "shadowrocket-proxies";

export interface ShareCopyFormatEntry {
  copyActionKey: TranslationKey;
  label: string;
  value: ShareCopyFormat;
}

export const shareCopyFormats: readonly ShareCopyFormatEntry[] = [
  { value: "uri-list", label: "URI list", copyActionKey: "shares.actions.copyUriList" },
  { value: "mihomo-proxies", label: "Mihomo", copyActionKey: "shares.actions.copyMihomo" },
  { value: "sing-box-outbounds", label: "sing-box", copyActionKey: "shares.actions.copySingBox" },
  { value: "shadowrocket-proxies", label: "Shadowrocket", copyActionKey: "shares.actions.copyShadowrocket" },
];

export function shareUrlWithFormat(publicUrl: string, format: ShareCopyFormat): string {
  const fragmentIndex = publicUrl.indexOf("#");
  const fragment = fragmentIndex >= 0 ? publicUrl.slice(fragmentIndex) : "";
  const urlWithoutFragment = fragmentIndex >= 0 ? publicUrl.slice(0, fragmentIndex) : publicUrl;
  const queryIndex = urlWithoutFragment.indexOf("?");
  const path = queryIndex >= 0 ? urlWithoutFragment.slice(0, queryIndex) : urlWithoutFragment;
  const query = queryIndex >= 0 ? urlWithoutFragment.slice(queryIndex + 1) : "";
  const params = new URLSearchParams(query);
  params.set("format", format);
  return `${path}?${params.toString()}${fragment}`;
}
