import type { TranslationKey } from "~/shared/i18n/context";

export type ShareCopyFormat = "base64" | "uri-list" | "mihomo-proxies" | "sing-box-outbounds" | "shadowrocket-proxies";

export interface ShareCopyFormatEntry {
  copyActionKey: TranslationKey;
  label: string;
  value: ShareCopyFormat;
}

export const shareCopyFormats: readonly ShareCopyFormatEntry[] = [
  { value: "base64", label: "Universal subscription (Base64)", copyActionKey: "shares.actions.copyBase64" },
  { value: "uri-list", label: "URI list", copyActionKey: "shares.actions.copyUriList" },
  { value: "mihomo-proxies", label: "Mihomo", copyActionKey: "shares.actions.copyMihomo" },
  { value: "sing-box-outbounds", label: "sing-box", copyActionKey: "shares.actions.copySingBox" },
  { value: "shadowrocket-proxies", label: "Shadowrocket", copyActionKey: "shares.actions.copyShadowrocket" },
];

export function shareUrlWithFormat(publicUrl: string, format: ShareCopyFormat, filename?: string): string {
  const fragmentIndex = publicUrl.indexOf("#");
  const fragment = fragmentIndex >= 0 ? publicUrl.slice(fragmentIndex) : "";
  const urlWithoutFragment = fragmentIndex >= 0 ? publicUrl.slice(0, fragmentIndex) : publicUrl;
  const queryIndex = urlWithoutFragment.indexOf("?");
  const currentPath = queryIndex >= 0 ? urlWithoutFragment.slice(0, queryIndex) : urlWithoutFragment;
  const path = filename ? publicSharePathWithFilename(currentPath, filename) : currentPath;
  const query = queryIndex >= 0 ? urlWithoutFragment.slice(queryIndex + 1) : "";
  const params = new URLSearchParams(query);
  params.set("format", format);
  return `${path}?${params.toString()}${fragment}`;
}

function publicSharePathWithFilename(path: string, filename: string): string {
  const markerIndex = path.lastIndexOf("/s/");
  if (markerIndex < 0) return path;
  const prefixEnd = markerIndex + 3;
  const suffix = path.slice(prefixEnd);
  const separatorIndex = suffix.indexOf("/");
  const id = separatorIndex >= 0 ? suffix.slice(0, separatorIndex) : suffix;
  return `${path.slice(0, prefixEnd)}${id}/${encodeURIComponent(filename)}`;
}
