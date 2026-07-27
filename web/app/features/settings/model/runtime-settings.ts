import type { RuntimeSettingsInput } from "~/shared/api/client";

export const defaultRuntimeSettings: RuntimeSettingsInput = {
  remote_defaults: {
    timeout_ms: 15000,
  },
  probe_defaults: {
    method: "url_test",
    core: "sing-box",
    url: "http://www.gstatic.com/generate_204",
    ntp_server: "time.apple.com",
    timeout_ms: 5000,
    attempts: 1,
    concurrency: 10,
    cache_ttl_seconds: 0,
  },
  cache_defaults: {
    remote_fetch_ttl_seconds: 0,
    subscription_traffic_ttl_seconds: 60,
    subscription_render_ttl_seconds: 0,
    file_render_ttl_seconds: 0,
  },
};

export function completeRuntimeSettings(settings: RuntimeSettingsInput | undefined): RuntimeSettingsInput {
  return {
    remote_defaults: {
      ...defaultRuntimeSettings.remote_defaults,
      ...(settings?.remote_defaults ?? {}),
    },
    probe_defaults: {
      ...defaultRuntimeSettings.probe_defaults,
      ...(settings?.probe_defaults ?? {}),
      core: "sing-box",
    },
    cache_defaults: {
      ...defaultRuntimeSettings.cache_defaults,
      ...(settings?.cache_defaults ?? {}),
    },
  };
}
