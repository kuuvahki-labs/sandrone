export const maxPublicConvertContentBytes = 64 << 10;

export type ConvertLinkSource =
  | { kind: "url"; value: string }
  | { kind: "content"; value: string };

export type ConvertLinkResponse = "raw" | "json";

export interface ConvertLinkInput {
  fromFormat?: string;
  publicBaseUrl: string;
  response: ConvertLinkResponse;
  source: ConvertLinkSource;
  toFormat: string;
}

export type ConvertLinkValidationError =
  | "source_required"
  | "url_invalid"
  | "url_scheme"
  | "content_too_large"
  | "to_format_required";

export function validateConvertLinkInput(input: ConvertLinkInput): ConvertLinkValidationError | null {
  const sourceValue = input.source.value;
  if (!sourceValue.trim()) return "source_required";
  if (!input.toFormat.trim()) return "to_format_required";
  if (input.source.kind === "content") {
    return utf8Length(sourceValue) > maxPublicConvertContentBytes ? "content_too_large" : null;
  }
  let url: URL;
  try {
    url = new URL(sourceValue.trim());
  } catch {
    return "url_invalid";
  }
  return url.protocol === "http:" || url.protocol === "https:" ? null : "url_scheme";
}

export function buildConvertLink(input: ConvertLinkInput): string {
  const validationError = validateConvertLinkInput(input);
  if (validationError) {
    throw new Error(validationError);
  }
  const params = new URLSearchParams();
  params.set(input.source.kind, input.source.kind === "url" ? input.source.value.trim() : input.source.value);
  const fromFormat = input.fromFormat?.trim();
  if (fromFormat) params.set("from_format", fromFormat);
  params.set("to_format", input.toFormat.trim());
  if (input.response === "json") params.set("response", "json");
  const base = input.publicBaseUrl.trim().replace(/\/+$/, "");
  return `${base}/convert?${params.toString()}`;
}

function utf8Length(value: string): number {
  return new TextEncoder().encode(value).byteLength;
}
