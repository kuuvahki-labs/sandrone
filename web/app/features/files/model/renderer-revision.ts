import { arrayField, asRecord } from "~/shared/resources/model-fields";

export function rendererRevisionFromInspect(
  value: unknown,
  rendererFormat: string,
): string | undefined {
  const capabilities = arrayField(asRecord(asRecord(value).capabilities).capabilities);
  const revisions = new Set<string>();

  for (const capabilityValue of capabilities) {
    const capability = asRecord(capabilityValue);
    if (capability.direction !== "render" || capability.format !== rendererFormat) {
      continue;
    }
    for (const fieldName of ["fields", "lossy", "raw_only"] as const) {
      for (const fieldValue of arrayField(capability[fieldName])) {
        const revision = asRecord(asRecord(fieldValue).source_ref).revision;
        if (typeof revision !== "string") {
          continue;
        }
        const trimmed = revision.trim();
        if (trimmed) {
          revisions.add(trimmed);
        }
      }
    }
  }

  return revisions.size === 1 ? revisions.values().next().value : undefined;
}
