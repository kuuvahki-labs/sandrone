import { act, renderHook, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { ApiError } from "~/shared/api/client";
import type { Translator } from "~/shared/i18n/context";

import { useResourceList } from "./use-resource-list";

const t: Translator = (key) => key === "errors.serviceUnavailable" ? "service unavailable" : key;
const ignoreNotice = () => undefined;

describe("useResourceList", () => {
  it("loads on mount, exposes loading, and supports an awaitable reload", async () => {
    const load = vi.fn()
      .mockResolvedValueOnce({ values: ["first"] })
      .mockResolvedValueOnce({ values: ["second"] });
    const { result } = renderHook(() => useResourceList({
      load,
      map: mapValues,
      showNotice: ignoreNotice,
      t,
    }));

    expect(result.current.loading).toBe(true);
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.items).toEqual(["first"]);

    await act(async () => {
      await result.current.reload();
    });
    expect(result.current.items).toEqual(["second"]);
    expect(load).toHaveBeenCalledTimes(2);
  });

  it("lets only the latest concurrent reload write items", async () => {
    const older = deferred<unknown>();
    const newer = deferred<unknown>();
    const load = vi.fn()
      .mockResolvedValueOnce({ values: ["initial"] })
      .mockReturnValueOnce(older.promise)
      .mockReturnValueOnce(newer.promise);
    const { result } = renderHook(() => useResourceList({
      load,
      map: mapValues,
      showNotice: ignoreNotice,
      t,
    }));
    await waitFor(() => expect(result.current.items).toEqual(["initial"]));

    let olderReload!: Promise<void>;
    let newerReload!: Promise<void>;
    act(() => {
      olderReload = result.current.reload();
      newerReload = result.current.reload();
    });
    await act(async () => {
      newer.resolve({ values: ["newer"] });
      await newerReload;
    });
    await act(async () => {
      older.resolve({ values: ["older"] });
      await olderReload;
    });

    expect(result.current.items).toEqual(["newer"]);
    expect(result.current.loading).toBe(false);
  });

  it("invalidates the active generation on unmount", async () => {
    const request = deferred<unknown>();
    const showNotice = vi.fn();
    const { unmount } = renderHook(() => useResourceList({
      load: () => request.promise,
      map: mapValues,
      showNotice,
      t,
    }));

    unmount();
    request.reject(new Error("late failure"));
    await request.promise.catch(() => undefined);
    await Promise.resolve();

    expect(showNotice).not.toHaveBeenCalled();
  });

  it("clears items and reports the latest Error message", async () => {
    const load = vi.fn()
      .mockResolvedValueOnce({ values: ["initial"] })
      .mockRejectedValueOnce(new Error("request failed"));
    const showNotice = vi.fn();
    const { result } = renderHook(() => useResourceList({ load, map: mapValues, showNotice, t }));
    await waitFor(() => expect(result.current.items).toEqual(["initial"]));

    await act(async () => {
      await result.current.reload();
    });

    expect(result.current.items).toEqual([]);
    expect(result.current.loading).toBe(false);
    expect(showNotice).toHaveBeenCalledOnce();
    expect(showNotice).toHaveBeenCalledWith("request failed", "error");
  });

  it("uses the translated fallback for a latest non-Error failure", async () => {
    const load = vi.fn()
      .mockResolvedValueOnce({ values: ["initial"] })
      .mockRejectedValueOnce("offline");
    const showNotice = vi.fn();
    const { result } = renderHook(() => useResourceList({ load, map: mapValues, showNotice, t }));
    await waitFor(() => expect(result.current.items).toEqual(["initial"]));

    await act(async () => {
      await result.current.reload();
    });

    expect(result.current.items).toEqual([]);
    expect(showNotice).toHaveBeenCalledWith("service unavailable", "error");
  });

  it("keeps existing items and stays silent for a 401", async () => {
    const load = vi.fn()
      .mockResolvedValueOnce({ values: ["initial"] })
      .mockRejectedValueOnce(new ApiError(401, "unauthorized", "unauthorized"));
    const showNotice = vi.fn();
    const { result } = renderHook(() => useResourceList({ load, map: mapValues, showNotice, t }));
    await waitFor(() => expect(result.current.items).toEqual(["initial"]));

    await act(async () => {
      await result.current.reload();
    });

    expect(result.current.items).toEqual(["initial"]);
    expect(result.current.loading).toBe(false);
    expect(showNotice).not.toHaveBeenCalled();
  });

  it("does not let a stale error or finally overwrite the latest generation", async () => {
    const stale = deferred<unknown>();
    const latest = deferred<unknown>();
    const load = vi.fn()
      .mockResolvedValueOnce({ values: ["initial"] })
      .mockReturnValueOnce(stale.promise)
      .mockReturnValueOnce(latest.promise);
    const showNotice = vi.fn();
    const { result } = renderHook(() => useResourceList({ load, map: mapValues, showNotice, t }));
    await waitFor(() => expect(result.current.items).toEqual(["initial"]));

    let staleReload!: Promise<void>;
    let latestReload!: Promise<void>;
    act(() => {
      staleReload = result.current.reload();
      latestReload = result.current.reload();
    });
    await act(async () => {
      stale.reject(new Error("stale failure"));
      await staleReload;
    });

    expect(result.current.items).toEqual(["initial"]);
    expect(result.current.loading).toBe(true);
    expect(showNotice).not.toHaveBeenCalled();

    await act(async () => {
      latest.resolve({ values: ["latest"] });
      await latestReload;
    });
    expect(result.current.items).toEqual(["latest"]);
    expect(result.current.loading).toBe(false);
  });
});

function mapValues(value: unknown): string[] {
  return (value as { values: string[] }).values;
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason: unknown) => void;
  const promise = new Promise<T>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise;
    reject = rejectPromise;
  });
  return { promise, reject, resolve };
}
