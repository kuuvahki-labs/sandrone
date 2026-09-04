import { arrayField, asRecord, stringField } from "~/shared/resources/model-fields";
import type { PreviewWarning } from "~/shared/resources/types";

export function warningsFromAPI(value: unknown): PreviewWarning[] {
  return arrayField(value).map(warningFromAPI).filter((warning) => warning.code || warning.message);
}

function warningFromAPI(value: unknown): PreviewWarning {
  const item = asRecord(value);
  return {
    ...item,
    code: stringField(item.code),
    message: stringField(item.message),
    node: stringField(item.node) || undefined,
    field: stringField(item.field) || undefined,
    source: stringField(item.source) || undefined,
    target: stringField(item.target) || undefined,
  };
}
