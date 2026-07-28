import { renderHook, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import type { ApiClient } from "~/shared/api/client";

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
