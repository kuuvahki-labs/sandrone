import type { ScheduledRefreshTarget } from "~/shared/api/client";

export interface ScheduledRefreshResourceChoice extends ScheduledRefreshTarget {
  label: string;
}

export interface ScheduledRefreshTargetOption extends ScheduledRefreshResourceChoice {
  missing: boolean;
}

export function scheduledRefreshTargetOptions(
  configured: ScheduledRefreshTarget[],
  resources: ScheduledRefreshResourceChoice[],
): ScheduledRefreshTargetOption[] {
  const options = new Map<string, ScheduledRefreshTargetOption>();
  for (const resource of resources) {
    options.set(targetKey(resource), { ...resource, missing: false });
  }
  for (const target of configured) {
    const key = targetKey(target);
    if (!options.has(key)) {
      options.set(key, { ...target, label: target.name, missing: true });
    }
  }
  return [...options.values()].sort((left, right) =>
    left.kind.localeCompare(right.kind) || left.label.localeCompare(right.label),
  );
}

export function targetKey(target: ScheduledRefreshTarget): string {
  return `${target.kind}\u0000${target.name}`;
}
