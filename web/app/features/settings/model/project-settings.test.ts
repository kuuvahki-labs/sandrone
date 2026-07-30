import { describe, expect, it } from "vitest";

import {
  completeProjectSettings,
  defaultProjectSettings,
  settingsUpdateFromView,
} from "./project-settings";

describe("project settings model", () => {
  it("defaults automatic subscription traffic to disabled", () => {
    expect(defaultProjectSettings.subscriptions.auto_load_traffic).toBe(false);
    expect(defaultProjectSettings.appearance.locale).toBe("auto");
  });

  it("completes nested groups without losing defaults", () => {
    const completed = completeProjectSettings({
      subscriptions: { auto_load_traffic: true },
    });

    expect(completed.subscriptions.auto_load_traffic).toBe(true);
    expect(completed.http.listen).toBe("127.0.0.1:1137");
  });

  it("omits a redacted token unless replacement or clearing was requested", () => {
    expect(settingsUpdateFromView(defaultProjectSettings).http).not.toHaveProperty("token");
    expect(settingsUpdateFromView(defaultProjectSettings, "replacement").http.token).toBe("replacement");
    expect(settingsUpdateFromView(defaultProjectSettings, "").http.token).toBe("");
  });
});
