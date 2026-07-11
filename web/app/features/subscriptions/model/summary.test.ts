import { describe, expect, it } from "vitest";

import { subscriptionSummary } from "./summary";
import type { SubscriptionItem } from "./types";

describe("subscription model summary", () => {
  it("counts total, kinds, and warnings", () => {
    const items: SubscriptionItem[] = [
      subscription("remote-a", "remote", "ready"),
      subscription("remote-b", "remote", "warning"),
      subscription("local", "local", "ready"),
      subscription("collection", "collection", "ready"),
    ];

    expect(subscriptionSummary(items)).toEqual({
      total: 4,
      remote: 2,
      local: 1,
      collections: 1,
      warnings: 1,
    });
  });
});

function subscription(
  name: string,
  kind: SubscriptionItem["kind"],
  status: SubscriptionItem["status"],
): SubscriptionItem {
  return { kind, name, title: name, label: kind, status };
}
