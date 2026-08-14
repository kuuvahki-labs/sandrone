import { describe, expect, it } from "vitest";

import {
  completeProjectSettings,
  defaultProjectSettings,
  settingsUpdateFromView,
} from "./project-settings";

describe("project settings model", () => {
  it("defaults automatic subscription traffic to disabled", () => {
    expect(defaultProjectSettings.subscriptions.auto_load_traffic).toBe(false);
    expect(defaultProjectSettings.remote_defaults.user_agent).toBe("");
    expect(defaultProjectSettings.appearance.locale).toBe("auto");
    expect(defaultProjectSettings.probe_defaults.url).toBe("https://cp.cloudflare.com");
    expect(defaultProjectSettings.scheduled_refresh).toEqual({
      enabled: false,
      schedule: "@every 10m",
      targets: [],
    });
  });

  it("completes nested groups without losing defaults", () => {
    const completed = completeProjectSettings({
      subscriptions: { auto_load_traffic: true },
    });

    expect(completed.subscriptions.auto_load_traffic).toBe(true);
    expect(completed.http.listen).toBe("127.0.0.1:1137");
  });

  it("serializes only server-owned startup settings", () => {
    const update = settingsUpdateFromView(defaultProjectSettings);

    expect(update.http).toEqual({ listen: "127.0.0.1:1137" });
    expect(update.mcp).not.toHaveProperty("transport");
    expect(update).not.toHaveProperty("webui");
    expect(update.scheduled_refresh).toEqual(defaultProjectSettings.scheduled_refresh);
  });
});
