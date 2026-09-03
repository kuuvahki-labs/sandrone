import { renderHook, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import type { ApiClient } from "~/shared/api/client";
import type { Translator } from "~/shared/i18n/context";

import { useSharesResource } from "./use-shares-resource";

const t: Translator = (key) => key;
const ignoreNotice = () => undefined;

describe("share resource data", () => {
  it("uses publicBaseUrl and the default resource order", async () => {
    const listShares = vi.fn().mockResolvedValue({ shares: [
      { created_at: "2026-01-01T00:00:00Z", id: "z", target_kind: "file", target_name: "z.yaml" },
      { created_at: "2026-02-01T00:00:00Z", id: "a", target_kind: "subscription", target_name: "a", target_format: "uri-list" },
    ] });
    const client = { listShares } as unknown as ApiClient;
    const { result } = renderHook(() => useSharesResource({
      client,
      publicBaseUrl: "https://public.example/",
      showNotice: ignoreNotice,
      t,
    }));

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.items.map((item) => item.id)).toEqual(["a", "z"]);
    expect(result.current.items.map((item) => item.publicUrl)).toEqual([
      "https://public.example/s/a?format=uri-list",
      "https://public.example/s/z",
    ]);
  });

  it("rebuilds mapping and reloads when publicBaseUrl changes", async () => {
    const listShares = vi.fn().mockResolvedValue({ shares: [
      { id: "share", target_kind: "file", target_name: "profile.yaml" },
    ] });
    const client = { listShares } as unknown as ApiClient;
    const { rerender, result } = renderHook(
      ({ publicBaseUrl }) => useSharesResource({ client, publicBaseUrl, showNotice: ignoreNotice, t }),
      { initialProps: { publicBaseUrl: "https://first.example" } },
    );
    await waitFor(() => expect(result.current.items[0]?.publicUrl).toBe("https://first.example/s/share"));

    rerender({ publicBaseUrl: "https://second.example" });

    await waitFor(() => expect(result.current.items[0]?.publicUrl).toBe("https://second.example/s/share"));
    expect(listShares).toHaveBeenCalledTimes(2);
  });
});
