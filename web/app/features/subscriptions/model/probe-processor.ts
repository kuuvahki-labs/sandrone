import type { ProbeDefaultsInput } from "~/shared/api/client";
import { cleanParams, stringValue } from "~/shared/processors/model";

export type ProbeMethod = ProbeDefaultsInput["method"];

export function newProbeProcessorParams(): Record<string, unknown> {
  return {
    annotate: true,
    fail_mode: "drop",
  };
}

export function probeMethodForDisplay(
  params: Record<string, unknown>,
  defaults: ProbeDefaultsInput,
): ProbeMethod {
  const method = stringValue(params.method);
  if (method === "tcp_connect" || method === "udp_ntp" || method === "url_test") {
    return method;
  }
  return defaults.method;
}

export function probeMethodPatch(method: string): Record<string, unknown> {
  switch (method) {
    case "":
      return { method: undefined };
    case "tcp_connect":
      return {
        method,
        core: undefined,
        url: undefined,
        expected_status: undefined,
        ntp_server: undefined,
      };
    case "udp_ntp":
      return {
        method,
        url: undefined,
        expected_status: undefined,
      };
    default:
      return {
        method: "url_test",
        ntp_server: undefined,
      };
  }
}

export function sanitizeProbeParams(params: Record<string, unknown>): Record<string, unknown> {
  const out = cleanParams(params);
  delete out.layer;
  if (out.cache_ttl_seconds === 0) {
    delete out.cache_ttl_seconds;
  }

  switch (stringValue(out.method)) {
    case "tcp_connect":
      delete out.core;
      delete out.url;
      delete out.expected_status;
      delete out.ntp_server;
      break;
    case "udp_ntp":
      delete out.url;
      delete out.expected_status;
      break;
    case "url_test":
      delete out.ntp_server;
      break;
  }
  return out;
}
