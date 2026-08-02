export type SubscriptionCreateType = "remote" | "local" | "collection";
export type FileCreateSource = string;
export type ResourcePreviewOrigin = "edit" | "list";

export function subscriptionResourcePath(kind: SubscriptionCreateType, name: string): string {
  return `/subscriptions/${kind}/${encodeURIComponent(name)}`;
}

export function subscriptionEditPath(kind: SubscriptionCreateType, name: string): string {
  return `${subscriptionResourcePath(kind, name)}/edit`;
}

export function subscriptionPreviewPath(kind: SubscriptionCreateType, name: string, origin?: ResourcePreviewOrigin): string {
  return withPreviewOrigin(`${subscriptionResourcePath(kind, name)}/preview`, origin);
}

export function subscriptionNewPath(type: SubscriptionCreateType): string {
  return `/subscriptions/new?type=${type}`;
}

export function fileNewPath(source: FileCreateSource): string {
  return `/files/new?source=${source}`;
}

export function fileResourcePath(name: string): string {
  return `/files/${encodeURIComponent(name)}`;
}

export function fileEditPath(name: string): string {
  return `${fileResourcePath(name)}/edit`;
}

export function filePreviewPath(name: string, origin?: ResourcePreviewOrigin): string {
  return withPreviewOrigin(`${fileResourcePath(name)}/preview`, origin);
}

export function resourcePreviewOrigin(value: string | null | undefined): ResourcePreviewOrigin {
  return value === "list" ? "list" : "edit";
}

export function sourceNameFromUrl(url: string): string {
  if (!url) return "provider";
  try {
    const parsed = new URL(url);
    return parsed.hostname.replace(/^www\./, "");
  } catch {
    return "provider";
  }
}

export function decodeRouteParam(value = ""): string {
  try {
    return decodeURIComponent(value);
  } catch {
    return value;
  }
}

export function decodeResourceRouteParam(value = ""): string | null {
  const decoded = decodeRouteParam(value);
  const trimmed = decoded.trim();
  if (!trimmed || trimmed === "." || trimmed === ".." || trimmed.includes("/") || trimmed.includes("\\")) {
    return null;
  }
  return decoded;
}

export function subscriptionKind(value = ""): SubscriptionCreateType | null {
  return value === "remote" || value === "local" || value === "collection" ? value : null;
}

function withPreviewOrigin(path: string, origin?: ResourcePreviewOrigin): string {
  return origin ? `${path}?from=${origin}` : path;
}
