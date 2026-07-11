import { describe, expect, it } from "vitest";

import { completeRuntimeSettings, defaultRuntimeSettings } from "./runtime-settings";

describe("runtime settings model", () => {
  it("completes undefined settings with independent copies of every default group", () => {
    const completed = completeRuntimeSettings(undefined);

    expect(completed).toEqual(defaultRuntimeSettings);
    expect(completed.remote_defaults).not.toBe(defaultRuntimeSettings.remote_defaults);
    expect(completed.probe_defaults).not.toBe(defaultRuntimeSettings.probe_defaults);
    expect(completed.cache_defaults).not.toBe(defaultRuntimeSettings.cache_defaults);
  });

  it("shallowly overlays each partial group while preserving untouched defaults", () => {
    expect(completeRuntimeSettings({
      remote_defaults: { user_agent: "Sandrone Test" },
      probe_defaults: { attempts: 3, method: "tcp_connect" },
      cache_defaults: { remote_fetch_ttl_seconds: 120 },
    })).toEqual({
      remote_defaults: {
        timeout_ms: 15000,
        user_agent: "Sandrone Test",
      },
      probe_defaults: {
        layer: "protocol",
        method: "tcp_connect",
        core: "sing-box",
        url: "http://www.gstatic.com/generate_204",
        ntp_server: "time.apple.com",
        timeout_ms: 5000,
        attempts: 3,
        concurrency: 10,
        cache_ttl_seconds: 0,
      },
      cache_defaults: {
        remote_fetch_ttl_seconds: 120,
        subscription_traffic_ttl_seconds: 60,
      },
    });
  });
});
