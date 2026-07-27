export type RenderCacheMode = "inherit" | "disabled" | "custom";

export function renderCacheMode(value: number | undefined): RenderCacheMode {
  if (value === 0) return "disabled";
  if (typeof value === "number" && value > 0) return "custom";
  return "inherit";
}

export function renderCacheTTLFromForm(form: FormData): number | undefined {
  const mode = String(form.get("render_cache_mode") ?? "inherit");
  if (mode === "disabled") return 0;
  if (mode !== "custom") return undefined;
  const raw = String(form.get("render_cache_ttl_seconds") ?? "").trim();
  if (!raw) return undefined;
  const value = Number(raw);
  return Number.isFinite(value) ? value : undefined;
}
