import { act, renderHook, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import {
  defaultProjectSettings,
  defaultSettingsEnvelope,
  settingsUpdateFromView,
} from "~/features/settings/model/project-settings";
import type { ApiClient, SettingsEnvelope } from "~/shared/api/client";

import { useProjectSettings } from "./use-project-settings";

describe("useProjectSettings", () => {
  it("keeps traffic disabled until server settings load", async () => {
    const pending = deferred<SettingsEnvelope>();
    const client = { getSettings: vi.fn(() => pending.promise) } as unknown as ApiClient;
    const showNotice = vi.fn();
    const setLocaleMode = vi.fn();
    const { result } = renderHook(() => useProjectSettings({
      client,
      setLocaleMode,
      showNotice,
      t,
    }));

    expect(result.current.settingsLoaded).toBe(false);
    expect(result.current.effectiveSettings.subscriptions.auto_load_traffic).toBe(false);

    pending.resolve(defaultSettingsEnvelope({
      ...defaultProjectSettings,
      subscriptions: { auto_load_traffic: true },
    }));
    await waitFor(() => expect(result.current.settingsLoaded).toBe(true));
    expect(result.current.effectiveSettings.subscriptions.auto_load_traffic).toBe(true);
  });

  it("applies server appearance and rolls back an optimistic failed save", async () => {
    const loaded = defaultSettingsEnvelope({
      ...defaultProjectSettings,
      appearance: { theme_mode: "light", locale: "en-US" },
    });
    const client = {
      getSettings: vi.fn().mockResolvedValue(loaded),
      updateSettings: vi.fn().mockRejectedValue(new Error("save failed")),
    } as unknown as ApiClient;
    const setLocaleMode = vi.fn();
    const showNotice = vi.fn();
    const { result } = renderHook(() => useProjectSettings({
      client,
      setLocaleMode,
      showNotice,
      t,
    }));
    await waitFor(() => expect(result.current.settingsLoaded).toBe(true));

    const changed = {
      ...result.current.settings,
      appearance: { theme_mode: "dark", locale: "zh-CN" } as const,
    };
    await act(async () => {
      await expect(result.current.updateSettings(settingsUpdateFromView(changed))).rejects.toThrow("save failed");
    });

    expect(result.current.effectiveSettings.appearance).toEqual(loaded.effective.appearance);
    expect(setLocaleMode).toHaveBeenLastCalledWith("en-US");
  });
});

const t = ((key: string) => key) as never;

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((resolvePromise) => {
    resolve = resolvePromise;
  });
  return { promise, resolve };
}
