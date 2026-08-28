import { describe, expect, it } from "vitest";

import {
  subscriptionDefinitionFromAPI,
  subscriptionPreviewFromAPI,
  subscriptionsFromResourceList,
  subscriptionTrafficFromAPI,
} from "./codec";

describe("subscription model codec", () => {
  it("maps subscriptions from resource lists", () => {
    expect(subscriptionsFromResourceList({ items: [
      { name: "provider", display_name: "Provider Main", type: "remote", format: "uri-list", created_at: "2026-06-27T01:02:03.000Z", updated_at: "2026-06-27T04:05:06.000Z", meta: { description: "daily\nbackup" } },
      { name: "raw", type: "local", format: "mihomo", warning: "format guessed" },
      { name: "default", type: "collection", meta: { node_count: "12", source_count: 2 } },
    ] })).toEqual([
      expect.objectContaining({
        kind: "remote",
        name: "provider",
        displayName: "Provider Main",
        title: "Provider Main",
        label: "remote",
        status: "ready",
        description: "daily\nbackup",
        createdAt: "2026-06-27T01:02:03.000Z",
        updatedAt: "2026-06-27T04:05:06.000Z",
      }),
      expect.objectContaining({ kind: "local", name: "raw", label: "local", status: "warning" }),
      expect.not.objectContaining({ name: "default", nodeCount: 12, sourceCount: 2 }),
    ]);
  });

  it("maps collection definitions into editable subscription refs and processors", () => {
    expect(subscriptionDefinitionFromAPI({
      name: "default",
      display_name: "Default Collection",
      type: "collection",
      created_at: "2026-06-27T01:02:03.000Z",
    updated_at: "2026-06-27T04:05:06.000Z",
    snapshot_ttl_seconds: 300,
      inputs: [
        { name: "provider", type: "subscription", ref: { kind: "subscription", name: "provider" } },
        { name: "ignored-file", type: "ref", ref: { kind: "file", name: "default.yaml" } },
        { name: "warn", type: "subscription", ref: { kind: "subscription", name: "warn" } },
      ],
      processors: [{ name: "only hk", type: "filter", stage: "nodes", enabled: false, params: { action: "keep", field: "name", match: "regex", pattern: "HK" } }],
      meta: { description: "main group" },
    })).toMatchObject({
      name: "default",
      displayName: "Default Collection",
      kind: "collection",
      createdAt: "2026-06-27T01:02:03.000Z",
    updatedAt: "2026-06-27T04:05:06.000Z",
    snapshotTTLSeconds: 300,
      sourceRefs: ["provider", "warn"],
      processors: [{ name: "only hk", type: "filter", stage: "nodes", enabled: false, params: { action: "keep", field: "name", match: "regex", pattern: "HK" } }],
      meta: { description: "main group" },
    });
  });

  it("maps subscription preview results into node diffs without traffic", () => {
    expect(subscriptionPreviewFromAPI({
      subscription_name: "provider",
      format: "uri-list",
      before_count: 2,
      after_count: 1,
      status_counts: { modified: 1, removed: 1 },
      traffic: { used_bytes: 123 },
      nodes: [
        {
          runtime_id: "runtime-one",
          status: "modified",
          before: { name: "node-a", type: "ss", server: "example.com", port: 8388 },
          after: { name: "source-node-a", type: "ss", server: "example.com", port: 8388 },
          target_names: { shadowrocket: "source-node-a" },
        },
        {
          runtime_id: "runtime-two",
          status: "removed",
          before: { name: "node-b", type: "ss", server: "example.org", port: 8389 },
        },
      ],
      warnings: [{
        code: "quick_settings_warning",
        message: "left unchanged",
        node: "node-a",
        node_index: 7,
        node_context: {
          format: "mihomo",
          name: "node-a",
          raw: { name: "node-a", type: "ss", server: "example.com", port: 8388 },
        },
      }],
    })).toEqual({
      subscriptionName: "provider",
      format: "uri-list",
      beforeCount: 2,
      afterCount: 1,
      statusCounts: { added: 0, modified: 1, removed: 1, unchanged: 0 },
      nodes: [
        expect.objectContaining({
          runtimeId: "runtime-one",
          status: "modified",
          targetNames: { shadowrocket: "source-node-a" },
          before: expect.objectContaining({ name: "node-a", type: "ss", endpoint: "example.com:8388" }),
          after: expect.objectContaining({ name: "source-node-a", type: "ss", endpoint: "example.com:8388" }),
        }),
        expect.objectContaining({
          runtimeId: "runtime-two",
          status: "removed",
          before: expect.objectContaining({ name: "node-b", endpoint: "example.org:8389" }),
          after: undefined,
        }),
      ],
      warnings: [expect.objectContaining({
        code: "quick_settings_warning",
        message: "left unchanged",
        node: "node-a",
        node_context: expect.objectContaining({
          format: "mihomo",
          raw: expect.objectContaining({ server: "example.com" }),
        }),
        node_index: 7,
      })],
    });
    expect(subscriptionPreviewFromAPI({ traffic: { used_bytes: 123 } })).not.toHaveProperty("traffic");
  });

  it("decodes strict probe summaries without inventing zero durations", () => {
    const preview = subscriptionPreviewFromAPI({
      nodes: [
        {
          runtime_id: "runtime-alive",
          status: "unchanged",
          after: {
            name: "alive",
            meta: {
              "probe.alive": "true",
              "probe.duration_ms": "42",
              "probe.method": "url_test",
              "probe.checked_at": "2026-08-12T01:02:03Z",
            },
          },
        },
        {
          runtime_id: "runtime-available",
          status: "unchanged",
          after: {
            name: "available",
            meta: { "probe.alive": "true", "probe.duration_ms": "0" },
          },
        },
        {
          runtime_id: "runtime-failed",
          status: "unchanged",
          after: {
            name: "failed",
            meta: {
              "probe.alive": "false",
              "probe.duration_ms": "invalid",
              "probe.error_code": "probe_tcp_failed",
            },
          },
        },
        {
          runtime_id: "runtime-invalid",
          status: "unchanged",
          after: {
            name: "invalid",
            meta: { "probe.alive": "yes", "probe.duration_ms": "7" },
          },
        },
      ],
    });

    expect(preview.nodes[0].after).toMatchObject({
      probe: {
        alive: true,
        durationMs: 42,
        method: "url_test",
        checkedAt: "2026-08-12T01:02:03Z",
      },
      raw: expect.objectContaining({
        meta: expect.objectContaining({ "probe.duration_ms": "42" }),
      }),
    });
    expect(preview.nodes[1].after?.probe).toEqual({ alive: true });
    expect(preview.nodes[2].after?.probe).toEqual({ alive: false, errorCode: "probe_tcp_failed" });
    expect(preview.nodes[3].after).not.toHaveProperty("probe");
  });

  it("maps subscription traffic", () => {
    expect(subscriptionTrafficFromAPI({
    subscription_name: "provider",
    type: "remote",
    format: "uri-list",
      traffic: {
        source_name: "provider",
        source_url: "https://example.test/sub",
        observed_at: "2026-06-28T01:02:03Z",
        upload_bytes: 1024,
        download_bytes: 2048,
        used_bytes: 3072,
        total_bytes: 10240,
        remaining_bytes: 7168,
        expires_at: "2026-07-01T00:00:00Z",
        remaining_days: 3,
        reset_day: 14,
        app_url: "https://panel.example.test",
        plan_name: "VIP 1",
      },
    })).toEqual({
    subscriptionName: "provider",
    kind: "remote",
    format: "uri-list",
      traffic: expect.objectContaining({
        sourceName: "provider",
        sourceUrl: "https://example.test/sub",
        observedAt: "2026-06-28T01:02:03Z",
        uploadBytes: 1024,
        downloadBytes: 2048,
        usedBytes: 3072,
        totalBytes: 10240,
        remainingBytes: 7168,
        expiresAt: "2026-07-01T00:00:00Z",
        remainingDays: 3,
        resetDay: 14,
        appUrl: "https://panel.example.test",
        planName: "VIP 1",
      }),
    });
  });
});
