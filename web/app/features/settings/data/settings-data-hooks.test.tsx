import { act, renderHook, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { ApiClient } from "~/shared/api/client";
import { createTranslator } from "~/shared/i18n/context";

import { useBackupOperations } from "./use-backup-operations";
import { useScheduledRefreshStatus } from "./use-scheduled-refresh-status";
import { useVersionInfo } from "./use-version-info";

describe("useVersionInfo", () => {
  it("loads build identity", async () => {
    const getVersion = vi.fn().mockResolvedValue({
      name: "sandrone",
      revision: "0123456789abcdef",
      version: "0.1.0",
    });
    const client = { getVersion } as unknown as ApiClient;
    const { result } = renderHook(() => useVersionInfo({ client }));

    await waitFor(() => {
      expect(result.current).toEqual({
        revision: "0123456789abcdef",
        version: "0.1.0",
      });
    });
    expect(getVersion).toHaveBeenCalledTimes(1);
  });

  it("loads build identity without showing an error when unavailable", async () => {
    const getVersion = vi.fn().mockRejectedValue(new Error("offline"));
    const client = { getVersion } as unknown as ApiClient;
    const { result } = renderHook(() => useVersionInfo({ client }));

    await waitFor(() => expect(getVersion).toHaveBeenCalledTimes(1));
    expect(result.current).toEqual({ revision: undefined, version: undefined });
  });
});

describe("useScheduledRefreshStatus", () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it("loads independently and polls every 30 seconds", async () => {
    vi.useFakeTimers();
    const getScheduledRefreshStatus = vi.fn()
      .mockResolvedValueOnce({ enabled: true, running: false, last_success_count: 1, last_failure_count: 0, skipped_count: 0 })
      .mockResolvedValueOnce({ enabled: true, running: true, last_success_count: 1, last_failure_count: 0, skipped_count: 0 });
    const client = { getScheduledRefreshStatus } as unknown as ApiClient;
    const { result, unmount } = renderHook(() => useScheduledRefreshStatus(client));

    await act(async () => Promise.resolve());
    expect(getScheduledRefreshStatus).toHaveBeenCalledTimes(1);
    expect(result.current?.running).toBe(false);

    await act(async () => vi.advanceTimersByTimeAsync(30_000));
    expect(getScheduledRefreshStatus).toHaveBeenCalledTimes(2);
    expect(result.current?.running).toBe(true);

    unmount();
    await vi.advanceTimersByTimeAsync(30_000);
    expect(getScheduledRefreshStatus).toHaveBeenCalledTimes(2);
  });
});

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
    const { result } = renderHook(() => useBackupOperations({ client, reloadSettings: vi.fn(), showNotice: vi.fn(), t }));

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
    const { result } = renderHook(() => useBackupOperations({ client, reloadSettings: vi.fn(), showNotice, t }));

    await act(async () => result.current.downloadBackup());

    const anchor = click.mock.instances[0] as HTMLAnchorElement;
    expect(showNotice).toHaveBeenCalledWith("备份下载失败", "error");
    expect(revokeObjectURL).toHaveBeenCalledWith("blob:sandrone-backup");
    expect(document.body).not.toContainElement(anchor);
  });

  it("refreshes project settings after restoring a backup", async () => {
    const file = new Blob(["backup"], { type: "application/zip" });
    const client = {
      restoreBackup: vi.fn().mockResolvedValue(undefined),
    } as unknown as ApiClient;
    const reloadSettings = vi.fn().mockResolvedValue(undefined);
    const showNotice = vi.fn();
    const { result } = renderHook(() => useBackupOperations({ client, reloadSettings, showNotice, t }));

    await act(async () => result.current.restoreBackup(file));

    expect(client.restoreBackup).toHaveBeenCalledWith(file);
    expect(reloadSettings).toHaveBeenCalledWith(true);
    expect(showNotice).toHaveBeenCalledWith("备份恢复成功");
  });

  it("propagates restore failures without refreshing runtime settings", async () => {
    const failure = new Error("archive invalid");
    const client = {
      restoreBackup: vi.fn().mockRejectedValue(failure),
    } as unknown as ApiClient;
    const reloadSettings = vi.fn();
    const showNotice = vi.fn();
    const { result } = renderHook(() => useBackupOperations({ client, reloadSettings, showNotice, t }));

    await expect(act(async () => result.current.restoreBackup(new Blob(["invalid"])))).rejects.toBe(failure);

    expect(reloadSettings).not.toHaveBeenCalled();
    expect(showNotice).toHaveBeenCalledWith("archive invalid", "error");
    expect(showNotice).not.toHaveBeenCalledWith("备份恢复成功");
  });

  it("reports a post-restore refresh failure and still completes the restore", async () => {
    const client = {
      restoreBackup: vi.fn().mockResolvedValue(undefined),
    } as unknown as ApiClient;
    const reloadSettings = vi.fn().mockRejectedValue(new Error("reload unavailable"));
    const showNotice = vi.fn();
    const { result } = renderHook(() => useBackupOperations({ client, reloadSettings, showNotice, t }));

    await act(async () => result.current.restoreBackup(new Blob(["backup"])));

    expect(showNotice).toHaveBeenCalledWith("运行默认值加载失败", "error");
    expect(showNotice).toHaveBeenCalledWith("备份恢复成功");
  });
});
