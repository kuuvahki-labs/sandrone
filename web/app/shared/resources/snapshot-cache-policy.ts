export type SnapshotCacheMode = "inherit" | "disabled" | "custom";

export function snapshotCacheMode(value: number | undefined): SnapshotCacheMode {
  if (value === 0) return "disabled";
  if (typeof value === "number" && value > 0) return "custom";
  return "inherit";
}

export function snapshotTTLFromForm(form: FormData): number | undefined {
  const mode = String(form.get("snapshot_mode") ?? "inherit");
  if (mode === "disabled") return 0;
  if (mode !== "custom") return undefined;
  const raw = String(form.get("snapshot_ttl_seconds") ?? "").trim();
  if (!raw) return undefined;
  const value = Number(raw);
  return Number.isFinite(value) ? value : undefined;
}
