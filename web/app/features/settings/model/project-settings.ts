import type {
  SettingsEnvelope,
  SettingsUpdate,
  SettingsView,
} from "~/shared/api/client";

export type ProjectSettings = SettingsView;

export const defaultProjectSettings: SettingsView = {
  schema_version: 1,
  http: {
    listen: "127.0.0.1:1137",
    token_configured: false,
    token_required: false,
  },
  mcp: {
    transport: "stdio",
    path: "/mcp",
    allow_management_tools: false,
    max_output_bytes: 1 << 20,
  },
  webui: {
    static_dir: "",
  },
  log: {
    level: "info",
  },
  remote_defaults: {
    user_agent: "sandrone/0.1.0",
    proxy: "",
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
  appearance: {
    theme_mode: "dark",
    locale: "auto",
  },
  subscriptions: {
    auto_load_traffic: false,
  },
};

export function completeProjectSettings(settings?: Partial<SettingsView>): SettingsView {
  return {
    ...defaultProjectSettings,
    ...settings,
    http: { ...defaultProjectSettings.http, ...settings?.http },
    mcp: { ...defaultProjectSettings.mcp, ...settings?.mcp },
    webui: { ...defaultProjectSettings.webui, ...settings?.webui },
    log: { ...defaultProjectSettings.log, ...settings?.log },
    remote_defaults: { ...defaultProjectSettings.remote_defaults, ...settings?.remote_defaults },
    probe_defaults: { ...defaultProjectSettings.probe_defaults, ...settings?.probe_defaults },
    cache_defaults: { ...defaultProjectSettings.cache_defaults, ...settings?.cache_defaults },
    appearance: { ...defaultProjectSettings.appearance, ...settings?.appearance },
    subscriptions: { ...defaultProjectSettings.subscriptions, ...settings?.subscriptions },
  };
}

export function settingsUpdateFromView(view: SettingsView, token?: string | null): SettingsUpdate {
  return {
    schema_version: view.schema_version,
    http: {
      listen: view.http.listen,
      token_required: view.http.token_required,
      ...(token === undefined ? {} : { token }),
    },
    mcp: view.mcp,
    webui: view.webui,
    log: view.log,
    remote_defaults: view.remote_defaults,
    probe_defaults: view.probe_defaults,
    cache_defaults: view.cache_defaults,
    appearance: view.appearance,
    subscriptions: view.subscriptions,
  };
}

export function defaultSettingsEnvelope(settings: SettingsView = defaultProjectSettings): SettingsEnvelope {
  return {
    settings,
    effective: settings,
    overrides: {},
    restart_required: [],
  };
}

export function optimisticSettingsEnvelope(
  previous: SettingsEnvelope,
  update: SettingsUpdate,
): SettingsEnvelope {
  const settings = completeProjectSettings({
    ...update,
    http: {
      listen: update.http.listen,
      token_configured: update.http.token === undefined || update.http.token === null
        ? previous.settings.http.token_configured
        : update.http.token.length > 0,
      token_required: update.http.token_required,
    },
  });
  return {
    ...previous,
    settings,
    effective: {
      ...previous.effective,
      remote_defaults: settings.remote_defaults,
      probe_defaults: settings.probe_defaults,
      cache_defaults: settings.cache_defaults,
      appearance: settings.appearance,
      subscriptions: settings.subscriptions,
    },
  };
}
