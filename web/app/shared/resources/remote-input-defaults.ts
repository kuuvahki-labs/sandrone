import type { SettingsView } from "~/shared/api/client";

import type { RemoteInputDefaults } from "./types";

export function remoteInputDefaultsFromSettings(
  settings: Pick<SettingsView, "cache_defaults" | "remote_defaults">,
): RemoteInputDefaults {
  return {
    cacheTTLSeconds: settings.cache_defaults.remote_fetch_ttl_seconds,
    proxy: settings.remote_defaults.proxy,
    timeoutMS: settings.remote_defaults.timeout_ms,
    userAgent: settings.remote_defaults.user_agent,
  };
}
