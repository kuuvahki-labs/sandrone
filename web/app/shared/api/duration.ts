export function millisecondsToSecondsInput(value: unknown): string {
  return typeof value === "number" && Number.isFinite(value)
    ? String(value / 1000)
    : "";
}

export function secondsInputToMilliseconds(value: string): number | undefined {
  const trimmed = value.trim();
  if (!trimmed) return undefined;
  const seconds = Number(trimmed);
  return Number.isFinite(seconds) ? Math.round(seconds * 1000) : undefined;
}

export function secondsInputToMillisecondsOrZero(value: string): number {
  return secondsInputToMilliseconds(value) ?? 0;
}
