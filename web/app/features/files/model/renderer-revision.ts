import { arrayField, asRecord } from "~/shared/resources/model-fields";

export function rendererRevisionFromCapabilityIndex(
  value: unknown,
  rendererFormat: string,
): string | undefined {
  const capabilities = arrayField(asRecord(value).items);
  const revisions = new Set<string>();

  for (const capabilityValue of capabilities) {
    const capability = asRecord(capabilityValue);
    if (capability.direction !== "render" || capability.format !== rendererFormat) {
      continue;
    }
    for (const revision of arrayField(capability.revisions)) {
      if (typeof revision !== "string") {
        continue;
      }
      const trimmed = revision.trim();
      if (trimmed) {
        revisions.add(trimmed);
      }
    }
  }

  return revisions.size === 1 ? revisions.values().next().value : undefined;
}
