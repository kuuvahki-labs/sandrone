import { describe, expect, it } from "vitest";

import type { ProbeDefaultsInput } from "~/shared/api/client";

import {
  newProbeProcessorParams,
  probeMethodForDisplay,
  probeMethodPatch,
  sanitizeProbeParams,
} from "./probe-processor";

const defaults: ProbeDefaultsInput = {
  method: "udp_ntp",
  core: "mihomo",
  url: "https://probe.example/generate_204",
  ntp_server: "time.example",
  timeout_ms: 8000,
  attempts: 2,
  concurrency: 4,
  cache_ttl_seconds: 60,
};

describe("probe processor params", () => {
  it("creates only processor behavior defaults", () => {
    expect(newProbeProcessorParams()).toEqual({
      annotate: true,
      fail_mode: "drop",
    });
  });

  it("uses runtime defaults for display without persisting them", () => {
    const params = newProbeProcessorParams();

    expect(probeMethodForDisplay(params, defaults)).toBe("udp_ntp");
    expect(sanitizeProbeParams(params)).toEqual(params);
  });

  it("normalizes the legacy zero cache sentinel to omission", () => {
    expect(sanitizeProbeParams({ cache_ttl_seconds: 0, fail_mode: "keep" })).toEqual({
      fail_mode: "keep",
    });
  });

  it("preserves explicit overrides", () => {
    const params = {
      method: "url_test",
      core: "mihomo",
      url: "https://custom.example/generate_204",
      expected_status: "204",
      timeout_ms: 9000,
      attempts: 3,
      concurrency: 5,
      cache_ttl_seconds: 120,
      fail_mode: "error",
    };

    expect(sanitizeProbeParams(params)).toEqual(params);
  });

  it("clears only fields incompatible with an explicit method change", () => {
    expect(probeMethodPatch("")).toEqual({ method: undefined });
    expect(probeMethodPatch("tcp_connect")).toEqual({
      method: "tcp_connect",
      core: undefined,
      url: undefined,
      expected_status: undefined,
      ntp_server: undefined,
    });
    expect(probeMethodPatch("udp_ntp")).toEqual({
      method: "udp_ntp",
      url: undefined,
      expected_status: undefined,
    });
    expect(probeMethodPatch("url_test")).toEqual({
      method: "url_test",
      ntp_server: undefined,
    });
  });
});
