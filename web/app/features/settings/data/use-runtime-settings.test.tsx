import { act, renderHook, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { defaultRuntimeSettings } from "~/features/settings/model/runtime-settings";
import type { ApiClient, RuntimeSettingsInput } from "~/shared/api/client";
import { createTranslator } from "~/shared/i18n/context";

import { useRuntimeSettings } from "./use-runtime-settings";

const t = createTranslator("zh-CN");

describe("useRuntimeSettings", () => {
  it("loads runtime and version independently and saves the server result", async () => {
    const loaded = runtimeSettings("loaded-agent", 15000);
    const saved = runtimeSettings("saved-agent", 30000);
    const getRuntimeSettings = vi.fn().mockResolvedValue(loaded);
    const getVersion = vi.fn().mockResolvedValue({ name: "sandrone", version: "0.1.0" });
    const updateRuntimeSettings = vi.fn().mockResolvedValue({ ok: true });
    const showNotice = vi.fn();
    const client = { getRuntimeSettings, getVersion, updateRuntimeSettings } as unknown as ApiClient;

    const { result } = renderHook(() => useRuntimeSettings({ client, showNotice, t }));

    expect(result.current.runtimeSettings).toEqual(defaultRuntimeSettings);
    await waitFor(() => expect(result.current.runtimeSettings).toEqual(loaded));
    expect(result.current.version).toBe("0.1.0");
    expect(getRuntimeSettings).toHaveBeenCalledTimes(1);
    expect(getVersion).toHaveBeenCalledTimes(1);

    await act(async () => {
      await result.current.saveRuntimeSettings(saved);
    });

    expect(updateRuntimeSettings).toHaveBeenCalledWith(saved);
    expect(result.current.runtimeSettings).toEqual(saved);
    expect(showNotice).toHaveBeenCalledWith("设置已保存");
  });

  it("keeps a fresh restored result when the initial request resolves later", async () => {
    let resolveInitial: ((settings: RuntimeSettingsInput) => void) | undefined;
    const initial = new Promise<RuntimeSettingsInput>((resolve) => {
      resolveInitial = resolve;
    });
    const stale = runtimeSettings("stale-agent", 15000);
    const restored = runtimeSettings("restored-agent", 30000);
    const getRuntimeSettings = vi.fn()
      .mockReturnValueOnce(initial)
      .mockResolvedValueOnce(restored);
    const client = {
      getRuntimeSettings,
      getVersion: vi.fn().mockResolvedValue({ name: "sandrone", version: "0.1.0" }),
      restoreBackup: vi.fn().mockResolvedValue(undefined),
    } as unknown as ApiClient;
    const showNotice = vi.fn();
    const { result } = renderHook(() => useRuntimeSettings({ client, showNotice, t }));

    await waitFor(() => expect(getRuntimeSettings).toHaveBeenCalledTimes(1));
    await act(async () => {
      await result.current.restoreBackup(new Blob(["backup"], { type: "application/zip" }));
    });

    expect(getRuntimeSettings.mock.calls[1]).toEqual([{ fresh: true }]);
    expect(result.current.runtimeSettings).toEqual(restored);

    await act(async () => {
      resolveInitial?.(stale);
      await initial;
    });

    expect(result.current.runtimeSettings).toEqual(restored);
    expect(showNotice).not.toHaveBeenCalledWith("运行默认值加载失败", "error");
  });
});

function runtimeSettings(userAgent: string, timeout: number): RuntimeSettingsInput {
  return {
    remote_defaults: { user_agent: userAgent, timeout_ms: timeout },
    probe_defaults: {},
    cache_defaults: {},
  };
}
