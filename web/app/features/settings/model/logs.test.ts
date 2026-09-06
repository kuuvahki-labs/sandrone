import { describe, expect, it } from "vitest";

import type { LogEntry } from "~/shared/api/client";

import { filterLogs } from "./logs";

describe("log filtering", () => {
  const entries: LogEntry[] = ["error", "warn", "info", "debug", "info+1"].map((level, index) => ({
    id: 5 - index, time: "", level, message: "Fetch", attrs: { nested: { status: 503 } }, truncated: false,
  }));
  it("applies level presets while keeping all levels visible by default", () => {
    expect(filterLogs(entries, "all", "")).toEqual(entries);
    expect(filterLogs(entries, "warnings", "")).toEqual(entries.slice(0, 2));
    expect(filterLogs(entries, "error", "")).toEqual([entries[0]]);
    expect(filterLogs(entries, "debug", "")).toEqual([entries[3]]);
  });
  it("matches messages and nested fields case-insensitively without changing order", () => {
    expect(filterLogs(entries, "warnings", " FETCH ")).toEqual(entries.slice(0, 2));
    expect(filterLogs(entries, "error", "503")).toEqual([entries[0]]);
    expect(filterLogs(entries, "all", "absent")).toEqual([]);
  });
});
