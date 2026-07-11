import { act, renderHook } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { useNotice } from "./use-notice";

describe("useNotice", () => {
  afterEach(() => vi.useRealTimers());

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
