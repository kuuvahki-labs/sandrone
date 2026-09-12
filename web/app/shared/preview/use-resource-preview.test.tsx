import { act, renderHook, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { useResourcePreview } from "./use-resource-preview";

describe("useResourcePreview", () => {
  it("loads one preview automatically and resets it for an empty key", async () => {
    const loadPreview = vi.fn().mockResolvedValue({ body: "ready" });
    const { rerender, result } = renderHook(
      ({ resourceKey }: { resourceKey: string | undefined }) => useResourcePreview(resourceKey, loadPreview),
      { initialProps: { resourceKey: "profile" as string | undefined } },
    );

    await waitFor(() => expect(result.current.preview).toEqual({ body: "ready" }));
    expect(result.current.pending).toBe(false);
    expect(result.current.failed).toBe(false);
    expect(loadPreview).toHaveBeenCalledTimes(1);

    rerender({ resourceKey: undefined });

    expect(result.current).toMatchObject({ failed: false, pending: false, preview: undefined });
    expect(loadPreview).toHaveBeenCalledTimes(1);
  });

  it("marks a null preview as failed", async () => {
    const loadPreview = vi.fn().mockResolvedValue(null);
    const { result } = renderHook(() => useResourcePreview("profile", loadPreview));

    await waitFor(() => expect(result.current.pending).toBe(false));

    expect(result.current.failed).toBe(true);
    expect(result.current.preview).toBeUndefined();
    expect(loadPreview).toHaveBeenCalledTimes(1);
  });

  it("keeps the previous preview visible while refreshing", async () => {
    const refresh = deferred<{ body: string } | null>();
    const loadPreview = vi.fn()
      .mockResolvedValueOnce({ body: "initial" })
      .mockImplementationOnce(() => refresh.promise);
    const { result } = renderHook(() => useResourcePreview("profile", loadPreview));
    await waitFor(() => expect(result.current.preview).toEqual({ body: "initial" }));

    act(() => result.current.refreshPreview());

    expect(result.current.pending).toBe(true);
    expect(result.current.failed).toBe(false);
    expect(result.current.preview).toEqual({ body: "initial" });
    await act(async () => refresh.resolve({ body: "refreshed" }));
    await waitFor(() => expect(result.current.pending).toBe(false));
    expect(result.current.preview).toEqual({ body: "refreshed" });
    expect(loadPreview).toHaveBeenCalledTimes(2);
  });

  it("uses the force loader only for an explicit refresh", async () => {
    const loadPreview = vi.fn().mockResolvedValue({ body: "initial" });
    const refreshPreview = vi.fn().mockResolvedValue({ body: "forced" });
    const { result } = renderHook(() => useResourcePreview("profile", loadPreview, refreshPreview));
    await waitFor(() => expect(result.current.preview).toEqual({ body: "initial" }));

    act(() => result.current.refreshPreview());

    await waitFor(() => expect(result.current.preview).toEqual({ body: "forced" }));
    expect(loadPreview).toHaveBeenCalledTimes(1);
    expect(refreshPreview).toHaveBeenCalledTimes(1);
  });

  it("keeps the visible preview and finishes pending when a refresh rejects", async () => {
    const loadPreview = vi.fn().mockResolvedValue({ body: "initial" });
    const refreshLoader = vi.fn().mockRejectedValue(new Error("preview unavailable"));
    const { result } = renderHook(() => useResourcePreview("profile", loadPreview, refreshLoader));
    await waitFor(() => expect(result.current.preview).toEqual({ body: "initial" }));

    act(() => result.current.refreshPreview());

    await waitFor(() => expect(result.current.pending).toBe(false));
    expect(result.current.preview).toEqual({ body: "initial" });
    expect(result.current.failed).toBe(true);
  });

  it("ignores an old refresh while the next resource is loading", async () => {
    const refresh = deferred<{ body: string } | null>();
    const next = deferred<{ body: string } | null>();
    const loadPreview = vi.fn()
      .mockResolvedValueOnce({ body: "first" })
      .mockImplementationOnce(() => next.promise);
    const refreshLoader = vi.fn(() => refresh.promise);
    const { rerender, result } = renderHook(
      ({ resourceKey }: { resourceKey: string }) => useResourcePreview(resourceKey, loadPreview, refreshLoader),
      { initialProps: { resourceKey: "first" } },
    );
    await waitFor(() => expect(result.current.preview).toEqual({ body: "first" }));
    act(() => result.current.refreshPreview());
    rerender({ resourceKey: "second" });

    await act(async () => refresh.resolve({ body: "stale" }));

    expect(result.current.preview).toBeUndefined();
    expect(result.current.pending).toBe(true);
    await act(async () => next.resolve({ body: "second" }));
    expect(result.current.preview).toEqual({ body: "second" });
    expect(result.current.pending).toBe(false);
  });

  it("does not overwrite the next resource with an old refresh", async () => {
    const refresh = deferred<{ body: string } | null>();
    const loadPreview = vi.fn()
      .mockResolvedValueOnce({ body: "first" })
      .mockResolvedValueOnce({ body: "second" });
    const refreshLoader = vi.fn(() => refresh.promise);
    const { rerender, result } = renderHook(
      ({ resourceKey }: { resourceKey: string }) => useResourcePreview(resourceKey, loadPreview, refreshLoader),
      { initialProps: { resourceKey: "first" } },
    );
    await waitFor(() => expect(result.current.preview).toEqual({ body: "first" }));
    act(() => result.current.refreshPreview());
    rerender({ resourceKey: "second" });
    await waitFor(() => expect(result.current.preview).toEqual({ body: "second" }));

    await act(async () => refresh.resolve({ body: "stale" }));

    expect(result.current.preview).toEqual({ body: "second" });
    expect(result.current.failed).toBe(false);
  });

  it("keeps the newest refresh when requests finish out of order", async () => {
    const first = deferred<{ body: string } | null>();
    const second = deferred<{ body: string } | null>();
    const loadPreview = vi.fn().mockResolvedValue({ body: "initial" });
    const refreshLoader = vi.fn()
      .mockImplementationOnce(() => first.promise)
      .mockImplementationOnce(() => second.promise);
    const { result } = renderHook(() => useResourcePreview("profile", loadPreview, refreshLoader));
    await waitFor(() => expect(result.current.preview).toEqual({ body: "initial" }));
    act(() => result.current.refreshPreview());
    act(() => result.current.refreshPreview());
    await act(async () => second.resolve({ body: "newest" }));
    await act(async () => first.resolve(null));

    expect(result.current.preview).toEqual({ body: "newest" });
    expect(result.current.failed).toBe(false);
    expect(result.current.pending).toBe(false);
  });

  it("ignores a stale result after the resource key changes", async () => {
    const first = deferred<{ body: string } | null>();
    const second = deferred<{ body: string } | null>();
    const loadPreview = vi.fn()
      .mockImplementationOnce(() => first.promise)
      .mockImplementationOnce(() => second.promise);
    const { rerender, result } = renderHook(
      ({ resourceKey }: { resourceKey: string }) => useResourcePreview(resourceKey, loadPreview),
      { initialProps: { resourceKey: "first" } },
    );

    rerender({ resourceKey: "second" });
    expect(loadPreview).toHaveBeenCalledTimes(2);
    await act(async () => first.resolve({ body: "stale" }));
    expect(result.current.preview).toBeUndefined();
    expect(result.current.pending).toBe(true);

    await act(async () => second.resolve({ body: "current" }));
    await waitFor(() => expect(result.current.pending).toBe(false));
    expect(result.current.preview).toEqual({ body: "current" });
    expect(result.current.failed).toBe(false);
  });
});

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((resolvePromise) => {
    resolve = resolvePromise;
  });
  return { promise, resolve };
}
