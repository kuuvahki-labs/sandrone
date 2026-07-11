import type { FileSourceDetail } from "./types";

export function fileSourceSummary(source: FileSourceDetail = {}): string {
  switch (source.type) {
    case "remote":
      return "remote";
    case "inline":
    case "":
    case undefined:
      return "local";
    default:
      return source.type;
  }
}
