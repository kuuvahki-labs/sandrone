import { act, render, renderHook, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  defaultProjectSettings,
  defaultSettingsEnvelope,
  settingsUpdateFromView,
} from "~/features/settings/model/project-settings";
import type { ApiClient, SettingsEnvelope } from "~/shared/api/client";
import { I18nProvider } from "~/shared/i18n/context";

import type { SandroneContextValue } from "./provider/types";
import { useNotice } from "./provider/use-notice";
import { useProjectSettings } from "./provider/use-project-settings";
import { SandroneProvider, useSandrone } from "./sandrone-provider";

afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

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

describe("useNotice", () => {
  it("keeps notices visible concurrently and dismisses each one independently", () => {
    vi.useFakeTimers();
    const { result } = renderHook(() => useNotice());

    act(() => {
      result.current.showNotice("Saved");
    });
    act(() => vi.advanceTimersByTime(1000));
    act(() => {
      result.current.showNotice("Request failed", "error");
    });

    expect(result.current.notices).toEqual([
      { id: 0, message: "Saved", severity: "success" },
      { id: 1, message: "Request failed", severity: "error" },
    ]);

    act(() => vi.advanceTimersByTime(1600));

    expect(result.current.notices).toEqual([
      { id: 1, message: "Request failed", severity: "error" },
    ]);

    act(() => vi.advanceTimersByTime(1000));

    expect(result.current.notices).toEqual([]);
  });
});

type LegacyShareKey = "closeSheet" | "createShare" | "openShareSheet" | "shareTarget" | "sheet";
type NoLegacyShareKeys = [Extract<keyof SandroneContextValue, LegacyShareKey>] extends [never] ? true : false;

const contextHasNoLegacyShareKeys: NoLegacyShareKeys = true;
const expectedContextKeys = [
  "cancelDelete",
  "client",
  "confirmDelete",
  "deleteTarget",
  "effectiveSettings",
  "enterWithToken",
  "getFeature",
  "hasFeature",
  "needsToken",
  "notices",
  "publicBaseUrl",
  "reloadSettings",
  "reloadUiCapabilities",
  "requestDelete",
  "restartRequired",
  "saveBaseUrl",
  "setTokenInput",
  "settings",
  "settingsLoaded",
  "settingsOverrides",
  "showNotice",
  "signOut",
  "tokenInput",
  "uiCapabilities",
  "uiCapabilitiesLoaded",
  "updateSettings",
];

describe("SandroneProvider", () => {
  beforeEach(() => {
    localStorage.clear();
    vi.stubGlobal("fetch", vi.fn(async () => new Response(JSON.stringify({
      settings: defaultProjectSettings,
      effective: defaultProjectSettings,
      overrides: {},
      restart_required: [],
      features: [],
    }), { headers: { "content-type": "application/json" } })));
  });

  it("exposes only global auth, preference, notice, and delete contracts", () => {
    render(
      <I18nProvider>
        <SandroneProvider>
          <ContextProbe />
        </SandroneProvider>
      </I18nProvider>,
    );

    expect(contextHasNoLegacyShareKeys).toBe(true);
    expect(JSON.parse(screen.getByTestId("context-keys").textContent ?? "[]")).toEqual(expectedContextKeys);
  });
});

function ContextProbe() {
  const context = useSandrone();
  return <output data-testid="context-keys">{JSON.stringify(Object.keys(context).sort())}</output>;
}
