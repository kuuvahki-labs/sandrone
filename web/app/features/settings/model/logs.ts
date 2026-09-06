import type { LogEntry } from "~/shared/api/client";

export const LOG_FILTERS = ["all", "warnings", "error", "debug"] as const;
export type LogFilter = typeof LOG_FILTERS[number];
export const LOG_PAGE_SIZE = 100;

export function filterLogs(entries: LogEntry[], filter: LogFilter, query: string): LogEntry[] {
  const needle = query.trim().toLowerCase();
  return entries.filter((entry) => (filter === "all" || (filter === "warnings" ? entry.level === "warn" || entry.level === "error" : entry.level === filter))
    && (!needle || `${entry.message}\n${JSON.stringify(entry.attrs)}`.toLowerCase().includes(needle)));
}
