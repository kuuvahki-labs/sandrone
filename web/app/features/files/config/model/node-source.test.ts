import { describe, expect, it } from "vitest";

import {
  configNodePreviewFromSubscription,
  type ConfigNodePreviewInput,
} from "./node-source";

describe("config node source model", () => {
  it("uses only post-processor nodes and keeps sensitive preview fields out of editor state", () => {
    const routePreview = {
      subscriptionName: "provider",
      beforeCount: 3,
      afterCount: 3,
      statusCounts: { added: 1, modified: 1, removed: 1, unchanged: 1 },
      nodes: [
        {
          runtimeId: "runtime-modified",
          status: "modified",
          before: {
            name: "old-hk",
            type: "ss",
            endpoint: "old.example:8388",
            raw: { password: "before-secret", token: "before-token" },
          },
          after: {
            name: "hk",
            type: "ss",
            endpoint: "hk.example:8388",
            raw: {
              name: "hk",
              password: "secret",
              token: "modified-token",
              server: "hk.example",
              port: 8388,
            },
          },
        },
        {
          runtimeId: "runtime-removed",
          status: "removed",
          before: { name: "removed", type: "vmess", endpoint: "removed.example:443" },
        },
        {
          runtimeId: "runtime-added",
          status: "added",
          after: {
            name: "jp",
            type: "trojan",
            endpoint: "jp.example:443",
            raw: { password: "added-secret", token: "added-token" },
          },
        },
        {
          runtimeId: "runtime-stable",
          status: "unchanged",
          before: { name: "stable", type: "hysteria2", endpoint: "stable.example:8443" },
          after: { name: "stable", type: "hysteria2", endpoint: "stable.example:8443" },
        },
      ],
      warnings: [{
        code: "preview-warning",
        message: "check source",
        node_context: {
          raw: { password: "warning-secret", token: "warning-token" },
        },
      }],
    };
    const preview: ConfigNodePreviewInput = routePreview;

    const routeState = JSON.stringify(preview);
    expect(routeState).toContain('"before"');
    expect(routeState).toContain('"raw"');
    expect(routeState).toContain('"node_context"');
    expect(routeState).toContain("secret");
    expect(routeState).toContain("token");

    const result = configNodePreviewFromSubscription(preview);

    expect(result).toEqual({
      subscriptionName: "provider",
      nodes: [
        { key: "runtime-modified", name: "hk", type: "ss", endpoint: "hk.example:8388" },
        { key: "runtime-added", name: "jp", type: "trojan", endpoint: "jp.example:443" },
        { key: "runtime-stable", name: "stable", type: "hysteria2", endpoint: "stable.example:8443" },
      ],
      renderCandidates: [
        { key: "runtime-modified", name: "hk", type: "ss", endpoint: "hk.example:8388" },
        { key: "runtime-added", name: "jp", type: "trojan", endpoint: "jp.example:443" },
        { key: "runtime-stable", name: "stable", type: "hysteria2", endpoint: "stable.example:8443" },
      ],
      options: [
        { key: "runtime-modified", name: "hk", type: "ss", endpoint: "hk.example:8388" },
        { key: "runtime-added", name: "jp", type: "trojan", endpoint: "jp.example:443" },
        { key: "runtime-stable", name: "stable", type: "hysteria2", endpoint: "stable.example:8443" },
      ],
      warnings: [{ code: "preview-warning", message: "check source" }],
      duplicateNames: [],
      unnamedCount: 0,
    });

    const editorState = JSON.stringify(result);
    expect(editorState).not.toContain('"before"');
    expect(editorState).not.toContain('"raw"');
    expect(editorState).not.toContain('"node_context"');
    expect(editorState).not.toContain("secret");
    expect(editorState).not.toContain("token");
  });

  it("omits unnamed nodes and deduplicates selectable names without reordering the preview", () => {
    const preview: ConfigNodePreviewInput = {
      subscriptionName: "provider",
      nodes: [
        { runtimeId: "runtime-first", after: { name: "hk", type: "ss", endpoint: "one.example:1" } },
        { runtimeId: "runtime-unnamed", after: { name: "  ", type: "ss", endpoint: "unnamed.example:2" } },
        { runtimeId: "runtime-duplicate", after: { name: "hk", type: "vmess", endpoint: "two.example:3" } },
        { runtimeId: "runtime-last", after: { name: "jp", type: "trojan", endpoint: "three.example:4" } },
      ],
      warnings: [],
    };

    const result = configNodePreviewFromSubscription(preview);

    expect(result.nodes.map((node) => node.name)).toEqual(["hk", "hk", "jp"]);
    expect(result.renderCandidates.map((node) => node.name)).toEqual(["hk", "  ", "hk", "jp"]);
    expect(result.options.map((node) => node.name)).toEqual(["hk", "jp"]);
    expect(result.duplicateNames).toEqual(["hk"]);
    expect(result.unnamedCount).toBe(1);
  });

  it("marks complete Shadowrocket target-name coverage and keeps skipped nodes out of options", () => {
    const preview: ConfigNodePreviewInput = {
      subscriptionName: "provider",
      nodes: [
        { runtimeId: "skipped", after: { name: "dup", type: "ss", endpoint: "bad.example:1" }, targetNames: { shadowrocket: "" } },
        { runtimeId: "first", after: { name: "dup", type: "http", endpoint: "one.example:2" }, targetNames: { shadowrocket: "dup" } },
        { runtimeId: "second", after: { name: "dup", type: "http", endpoint: "two.example:3" }, targetNames: { shadowrocket: "dup (2)" } },
      ],
      warnings: [],
    };

    expect(configNodePreviewFromSubscription(preview).targetOptions?.shadowrocket).toEqual({
      coverage: "complete",
      options: [
        { key: "first", name: "dup", type: "http", endpoint: "one.example:2" },
        { key: "second", name: "dup (2)", type: "http", endpoint: "two.example:3" },
      ],
    });
  });

  it("keeps partial authoritative Shadowrocket names without exposing unmapped raw candidates", () => {
    const preview: ConfigNodePreviewInput = {
      subscriptionName: "provider",
      nodes: [
        { runtimeId: "mapped", after: { name: "US,1", type: "http", endpoint: "one.example:1" }, targetNames: { shadowrocket: "US，1" } },
        { runtimeId: "unmapped", after: { name: "raw unsupported", type: "unknown", endpoint: "two.example:2" } },
      ],
      warnings: [],
    };

    expect(configNodePreviewFromSubscription(preview).targetOptions?.shadowrocket).toEqual({
      coverage: "partial",
      options: [{ key: "mapped", name: "US，1", type: "http", endpoint: "one.example:1" }],
    });
  });

  it("represents an all-skipped complete Shadowrocket response without raw fallback", () => {
    const preview: ConfigNodePreviewInput = {
      subscriptionName: "provider",
      nodes: [
        { runtimeId: "one", after: { name: "raw one", type: "ss", endpoint: "one.example:1" }, targetNames: { shadowrocket: "" } },
        { runtimeId: "two", after: { name: "raw two", type: "ss", endpoint: "two.example:2" }, targetNames: { shadowrocket: "" } },
      ],
      warnings: [],
    };

    expect(configNodePreviewFromSubscription(preview).targetOptions?.shadowrocket).toEqual({
      coverage: "complete",
      options: [],
    });
  });
});
