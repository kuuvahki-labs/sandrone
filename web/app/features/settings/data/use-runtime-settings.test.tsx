import { act, renderHook, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { defaultRuntimeSettings } from "~/features/settings/model/runtime-settings";
import type { ApiClient, RuntimeSettingsInput } from "~/shared/api/client";
import { createTranslator } from "~/shared/i18n/context";

import { useRuntimeSettings } from "./use-runtime-settings";

const t = createTranslator("zh-CN");

describe("useRuntimeSettings", () => {
  it("loads and saves only runtime settings", async () => {
    const loaded = runtimeSettings("loaded-agent", 15000);
    const saved = runtimeSettings("saved-agent", 30000);
    const client = {
      getRuntimeSettings: vi.fn().mockResolvedValue(loaded),
      updateRuntimeSettings: vi.fn().mockResolvedValue({ ok: true }),
    } as unknown as ApiClient;
    const showNotice = vi.fn();
    const { result } = renderHook(() => useRuntimeSettings({ client, showNotice, t }));

    expect(result.current.runtimeSettings).toEqual(defaultRuntimeSettings);
    await waitFor(() => expect(result.current.runtimeSettings).toEqual(loaded));
    await act(async () => result.current.saveRuntimeSettings(saved));

    expect(client.getRuntimeSettings).toHaveBeenCalledTimes(1);
    expect(client.updateRuntimeSettings).toHaveBeenCalledWith(saved);
    expect(result.current.runtimeSettings).toEqual(saved);
    expect(showNotice).toHaveBeenCalledWith("设置已保存");
  });

  it("does not let an obsolete client request replace newer runtime settings", async () => {
    let resolveObsolete: ((settings: RuntimeSettingsInput) => void) | undefined;
    const obsoleteRequest = new Promise<RuntimeSettingsInput>((resolve) => {
      resolveObsolete = resolve;
    });
    const obsoleteClient = {
      getRuntimeSettings: vi.fn().mockReturnValue(obsoleteRequest),
    } as unknown as ApiClient;
    const current = runtimeSettings("current-agent", 30000);
    const currentClient = {
      getRuntimeSettings: vi.fn().mockResolvedValue(current),
    } as unknown as ApiClient;
    const showNotice = vi.fn();
    const { result, rerender } = renderHook(
      ({ client }) => useRuntimeSettings({ client, showNotice, t }),
      { initialProps: { client: obsoleteClient } },
    );

    await waitFor(() => expect(obsoleteClient.getRuntimeSettings).toHaveBeenCalledTimes(1));
    rerender({ client: currentClient });
    await waitFor(() => expect(result.current.runtimeSettings).toEqual(current));

    await act(async () => {
      resolveObsolete?.(runtimeSettings("obsolete-agent", 15000));
      await obsoleteRequest;
    });

    expect(result.current.runtimeSettings).toEqual(current);
  });
});

function runtimeSettings(userAgent: string, timeout: number): RuntimeSettingsInput {
  return {
    remote_defaults: { user_agent: userAgent, timeout_ms: timeout },
    probe_defaults: {},
    cache_defaults: {},
  };
}
