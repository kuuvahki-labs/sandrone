import { describe, expect, it } from "vitest";

import {
  fileDetailFromAPI,
  filePreviewFromAPI,
  filesFromResourceList,
  ruleSetCatalogFromAPI,
} from "./codec";

const files = [
  {
    name: "default.yaml",
    display_name: "Default Profile",
    type: "remote",
    created_at: "2026-06-27T01:02:03.000Z",
    updated_at: "2026-06-27T04:05:06.000Z",
    meta: { description: "main config\nfor mobile clients" },
  },
  {
    name: "scripted.yaml",
    type: "inline",
    processors: [{ type: "script", stage: "file", params: { source: { type: "file", name: "scripts/file.js" } } }],
  },
];

describe("file model codec", () => {
  it("maps only usable rule-set catalog fields", () => {
    expect(ruleSetCatalogFromAPI({
      items: [
        { name: "geosite-cn", url: "https://example.com/geosite-cn.srs", rule_kind: "domain", ignored: "metadata" },
        { name: "geoip-cn", url: "https://example.com/geoip-cn.srs", rule_kind: "ip" },
        { name: "mixed", url: "https://example.com/mixed.list", rule_kind: "mixed", reference_type: "RULE-SET" },
        { name: "domain-set", url: "https://example.com/domain.list", rule_kind: "domain", reference_type: "DOMAIN-SET" },
        { name: "invalid-reference", url: "https://example.com/invalid.list", rule_kind: "mixed", reference_type: "rule-set" },
        { name: "invalid-kind", url: "https://example.com/invalid.srs", rule_kind: "future" },
        { name: "missing-url", rule_kind: "domain" },
      ],
    })).toEqual({
      items: [
        { name: "geosite-cn", url: "https://example.com/geosite-cn.srs", ruleKind: "domain" },
        { name: "geoip-cn", url: "https://example.com/geoip-cn.srs", ruleKind: "ip" },
        { name: "mixed", url: "https://example.com/mixed.list", ruleKind: "mixed", referenceType: "RULE-SET" },
        { name: "domain-set", url: "https://example.com/domain.list", ruleKind: "domain", referenceType: "DOMAIN-SET" },
        { name: "invalid-reference", url: "https://example.com/invalid.list", ruleKind: "mixed" },
      ],
    });
  });

  it("maps files from resource lists", () => {
    expect(filesFromResourceList({ items: files })).toEqual([
      expect.objectContaining({
        name: "default.yaml",
        displayName: "Default Profile",
        title: "Default Profile",
        description: "main config\nfor mobile clients",
        kind: "",
        sourceType: "remote",
        sourceSummary: "remote",
        createdAt: "2026-06-27T01:02:03.000Z",
        updatedAt: "2026-06-27T04:05:06.000Z",
      }),
      expect.objectContaining({
        name: "scripted.yaml",
        kind: "",
        sourceType: "inline",
        sourceSummary: "local",
        processorCount: 1,
      }),
    ]);
  });

  it("maps file specs into editable file details", () => {
    expect(fileDetailFromAPI({
      name: "remote.yaml",
      kind: "static",
      display_name: "Remote Base",
    created_at: "2026-06-27T01:02:03.000Z",
    updated_at: "2026-06-27T04:05:06.000Z",
      source: { type: "remote", remote: { url: "https://example.com/base.yaml", user_agent: "Sandrone Web", timeout_ms: 9000 } },
      processors: [{ type: "script", stage: "file", params: { source: { type: "file", name: "scripts/file.js" } } }],
      meta: { description: "remote file" },
    })).toMatchObject({
      name: "remote.yaml",
      kind: "static",
      displayName: "Remote Base",
    createdAt: "2026-06-27T01:02:03.000Z",
    updatedAt: "2026-06-27T04:05:06.000Z",
      source: { type: "remote", remote: { url: "https://example.com/base.yaml", user_agent: "Sandrone Web", timeout_ms: 9000 } },
      processors: [{ type: "script", stage: "file", params: { source: { type: "file", name: "scripts/file.js" } } }],
      meta: { description: "remote file" },
      rawSpec: {
        name: "remote.yaml",
        kind: "static",
      },
    });
  });

  it("preserves implicit and explicit empty file sources as distinct values", () => {
    expect(fileDetailFromAPI({
      name: "implicit.yaml",
      kind: "mihomo",
      source: {},
    }).source).toEqual({});
    expect(fileDetailFromAPI({
      name: "empty.txt",
      kind: "static",
      source: { type: "inline", content: "" },
    }).source).toEqual({ type: "inline", content: "" });
  });

  it("keeps kind separate from source type in file summaries", () => {
    const [item] = filesFromResourceList({ items: [{ name: "client.json", target: "sing-box", type: "remote" }] });
    expect(item).toMatchObject({ kind: "sing-box", sourceType: "remote" });
  });

  it("preserves unknown and missing file kinds without coercion", () => {
    const unknown = fileDetailFromAPI({
      name: "future.json",
      kind: "future-client",
      source: { type: "inline", content: "{}" },
      config: { subscriptions: ["provider"], settings: { future: true } },
      future_field: { keep: true },
    });
    const missing = fileDetailFromAPI({ name: "legacy.txt", source: { type: "inline", content: "legacy" } });

    expect(unknown.kind).toBe("future-client");
    expect(unknown.rawSpec).toMatchObject({
      kind: "future-client",
      future_field: { keep: true },
    });
    expect(missing.kind).toBe("");
    expect(missing.rawSpec).toMatchObject({ name: "legacy.txt" });
    expect(missing.rawSpec).not.toHaveProperty("kind");
  });

  it("parses only the common config envelope and preserves settings presence", () => {
    const explicit = fileDetailFromAPI({
      name: "future.json",
      kind: "future-client",
      config: { subscriptions: ["provider"], settings: null },
    });
    const omitted = fileDetailFromAPI({
      name: "client.yaml",
      kind: "mihomo",
      config: { subscriptions: ["provider"] },
    });

    expect(explicit.config).toEqual({
      subscriptions: ["provider"],
      settingsPresent: true,
      settings: null,
    });
    expect(omitted.config).toEqual({
      subscriptions: ["provider"],
      settingsPresent: false,
    });
    expect(omitted.config).not.toHaveProperty("groups");
  });

  it("keeps known driver settings inside the common config envelope", () => {
    const detail = fileDetailFromAPI({
      name: "client.yaml",
      kind: "mihomo",
      source: {},
      config: {
        subscriptions: ["provider"],
        settings: { adaptive_groups: { type: "url-test", regions: [] }, groups: [], rule_sets: [], rules: [] },
      },
    });

    expect(detail.config).toEqual({
      subscriptions: ["provider"],
      settingsPresent: true,
      settings: {
        adaptive_groups: { type: "url-test", regions: [] },
        groups: [],
        rule_sets: [],
        rules: [],
      },
    });
  });

  it("maps persisted adaptive group settings including an empty region selection", () => {
    const detail = fileDetailFromAPI({
      name: "adaptive.yaml",
      kind: "mihomo",
      config: {
        settings: { adaptive_groups: {
          type: "load-balance",
        } },
      },
    });

    expect(detail.config).toEqual({
      settingsPresent: true,
      settings: { adaptive_groups: {
        type: "load-balance",
      } },
    });
  });

  it("maps file preview responses into final rendered content and warnings", () => {
    expect(filePreviewFromAPI({
      content_type: "application/yaml",
      body: "proxies: []\n",
      response: { content_type: "application/yaml" },
      warnings: [{ code: "file_script_warning", message: "left unchanged", source: "default.yaml" }],
    })).toEqual({
      contentType: "application/yaml",
      body: "proxies: []\n",
      response: { content_type: "application/yaml" },
      warnings: [expect.objectContaining({ code: "file_script_warning", message: "left unchanged", source: "default.yaml" })],
    });
  });
});
