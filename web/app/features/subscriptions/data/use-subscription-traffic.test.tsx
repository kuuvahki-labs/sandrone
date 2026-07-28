import { act, renderHook, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { subscriptions, subscriptionTraffic } from "~/features/subscriptions/test-data";

import { useSubscriptionTrafficByKey } from "./use-subscription-traffic";

describe("useSubscriptionTrafficByKey", () => {
  it("clears pending remote traffic and ignores its result when automatic loading is disabled", async () => {
    const pending = deferred<typeof subscriptionTraffic>();
    const loadSubscriptionTraffic = vi.fn<(name: string) => Promise<typeof subscriptionTraffic>>(() => pending.promise);
    const { rerender, result } = renderHook(
      ({ enabled }: { enabled: boolean }) => useSubscriptionTrafficByKey(subscriptions, enabled, loadSubscriptionTraffic),
      { initialProps: { enabled: true } },
    );

    await waitFor(() => expect(loadSubscriptionTraffic).toHaveBeenCalledTimes(2));
    expect(loadSubscriptionTraffic.mock.calls.map(([name]) => name)).toEqual(["provider", "warn"]);

    rerender({ enabled: false });
    expect(result.current).toEqual({});

    await act(async () => {
      pending.resolve({
        ...subscriptionTraffic,
        traffic: { uploadBytes: 1024, downloadBytes: 2048, usedBytes: 3072 },
      });
      await pending.promise;
    });

    expect(result.current).toEqual({});
  });
});

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((resolvePromise) => {
    resolve = resolvePromise;
  });
  return { promise, resolve };
}
