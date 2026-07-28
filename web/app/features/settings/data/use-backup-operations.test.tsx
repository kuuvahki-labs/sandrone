import { act, renderHook } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { ApiClient, RuntimeSettingsInput } from "~/shared/api/client";
import { createTranslator } from "~/shared/i18n/context";

import { useBackupOperations } from "./use-backup-operations";

const t = createTranslator("zh-CN");

describe("useBackupOperations", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("downloads through a temporary anchor and cleans up its object URL", async () => {
    const blob = new Blob(["backup"], { type: "application/zip" });
    const client = {
      downloadBackup: vi.fn().mockResolvedValue({ blob, filename: "server-backup.zip" }),
    } as unknown as ApiClient;
    const createObjectURL = vi.spyOn(URL, "createObjectURL").mockReturnValue("blob:sandrone-backup");
    const revokeObjectURL = vi.spyOn(URL, "revokeObjectURL").mockImplementation(() => undefined);
    const click = vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(() => undefined);
    const { result } = renderHook(() => useBackupOperations({ client, showNotice: vi.fn(), t }));

    await act(async () => result.current.downloadBackup());

    const anchor = click.mock.instances[0] as HTMLAnchorElement;
    expect(createObjectURL).toHaveBeenCalledWith(blob);
    expect(anchor.download).toBe("server-backup.zip");
    expect(anchor.href).toBe("blob:sandrone-backup");
    expect(revokeObjectURL).toHaveBeenCalledWith("blob:sandrone-backup");
    expect(document.body).not.toContainElement(anchor);
  });

  it("cleans up the temporary anchor and object URL when download triggering fails", async () => {
    const client = {
      downloadBackup: vi.fn().mockResolvedValue({
        blob: new Blob(["backup"], { type: "application/zip" }),
        filename: "server-backup.zip",
      }),
    } as unknown as ApiClient;
    vi.spyOn(URL, "createObjectURL").mockReturnValue("blob:sandrone-backup");
    const revokeObjectURL = vi.spyOn(URL, "revokeObjectURL").mockImplementation(() => undefined);
    const click = vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(() => {
      throw new Error("download blocked");
    });
    const showNotice = vi.fn();
    const { result } = renderHook(() => useBackupOperations({ client, showNotice, t }));

    await act(async () => result.current.downloadBackup());

    const anchor = click.mock.instances[0] as HTMLAnchorElement;
    expect(showNotice).toHaveBeenCalledWith("备份下载失败", "error");
    expect(revokeObjectURL).toHaveBeenCalledWith("blob:sandrone-backup");
    expect(document.body).not.toContainElement(anchor);
  });

  it("refreshes runtime settings after restoring a backup", async () => {
    const file = new Blob(["backup"], { type: "application/zip" });
    const client = {
      restoreBackup: vi.fn().mockResolvedValue(undefined),
      getRuntimeSettings: vi.fn().mockResolvedValue(runtimeSettings("restored", 30000)),
    } as unknown as ApiClient;
    const showNotice = vi.fn();
    const { result } = renderHook(() => useBackupOperations({ client, showNotice, t }));

    await act(async () => result.current.restoreBackup(file));

    expect(client.restoreBackup).toHaveBeenCalledWith(file);
    expect(client.getRuntimeSettings).toHaveBeenCalledWith({ fresh: true });
    expect(showNotice).toHaveBeenCalledWith("备份恢复成功");
  });

  it("propagates restore failures without refreshing runtime settings", async () => {
    const failure = new Error("archive invalid");
    const client = {
      restoreBackup: vi.fn().mockRejectedValue(failure),
      getRuntimeSettings: vi.fn(),
    } as unknown as ApiClient;
    const showNotice = vi.fn();
    const { result } = renderHook(() => useBackupOperations({ client, showNotice, t }));

    await expect(act(async () => result.current.restoreBackup(new Blob(["invalid"])))).rejects.toBe(failure);

    expect(client.getRuntimeSettings).not.toHaveBeenCalled();
    expect(showNotice).toHaveBeenCalledWith("archive invalid", "error");
    expect(showNotice).not.toHaveBeenCalledWith("备份恢复成功");
  });

  it("reports a post-restore refresh failure and still completes the restore", async () => {
    const client = {
      restoreBackup: vi.fn().mockResolvedValue(undefined),
      getRuntimeSettings: vi.fn().mockRejectedValue(new Error("reload unavailable")),
    } as unknown as ApiClient;
    const showNotice = vi.fn();
    const { result } = renderHook(() => useBackupOperations({ client, showNotice, t }));

    await act(async () => result.current.restoreBackup(new Blob(["backup"])));

    expect(showNotice).toHaveBeenCalledWith("运行默认值加载失败", "error");
    expect(showNotice).toHaveBeenCalledWith("备份恢复成功");
  });
});

function runtimeSettings(userAgent: string, timeout: number): RuntimeSettingsInput {
  return {
    remote_defaults: { user_agent: userAgent, timeout_ms: timeout },
    probe_defaults: {},
    cache_defaults: {},
  };
}
