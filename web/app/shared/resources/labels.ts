import type { ResourceOption } from "./types";

export function resourceTitle(item: ResourceOption): string {
  return item.title || item.name;
}

export function resourceSecondaryName(item: ResourceOption): string | undefined {
  return resourceTitle(item) === item.name ? undefined : item.name;
}

export function resourceOptionText(item: ResourceOption): string {
  const title = resourceTitle(item);
  const name = resourceSecondaryName(item);
  return name ? `${title} (${name})` : title;
}
