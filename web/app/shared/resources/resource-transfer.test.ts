import { describe, expect, it } from "vitest";

import {
  copyTransferredResource,
  parseResourceTransfer,
  renameTransferredResource,
  resourceNameIssue,
  ResourceTransferError,
  resourceTransferFilename,
  serializeResourceTransfer,
} from "./resource-transfer";

describe("single resource transfer", () => {
  it.each([
    ["subscription" as const, { name: "provider", type: "remote", remote: { url: "https://example.test/sub" }, processors: [{ type: "rename" }] }],
    ["file" as const, { name: "client.yaml", kind: "mihomo", source: { type: "inline", content: "mixed-port: 7890" }, config: { subscriptions: ["provider"] } }],
  ])("round-trips a complete %s definition", (resourceType, resource) => {
    const serialized = serializeResourceTransfer(resourceType, resource);

    expect(serialized.endsWith("\n")).toBe(true);
    expect(parseResourceTransfer(serialized, resourceType)).toEqual({ resourceType, resource });
  });

  it.each([
    ["not json", "invalid_json", "subscription"],
    [JSON.stringify({ resource_type: "future", resource: { name: "x" } }), "invalid_envelope", "subscription"],
    [JSON.stringify({ resource_type: "subscription", resource: {} }), "missing_name", "subscription"],
    [JSON.stringify({ resource_type: "subscription", resource: { name: "provider" } }), "type_mismatch", "file"],
  ] as const)("rejects invalid transfer input", (text, code, expectedType) => {
    try {
      parseResourceTransfer(text, expectedType);
      throw new Error("expected parseResourceTransfer to fail");
    } catch (error) {
      expect(error).toBeInstanceOf(ResourceTransferError);
      expect((error as ResourceTransferError).code).toBe(code);
    }
  });

  it("renames only the root and resets timestamps only for copies", () => {
    const source = {
      name: "source",
      created_at: "2025-01-01T00:00:00Z",
      updated_at: "2025-01-02T00:00:00Z",
      inputs: [{ name: "source", ref: { kind: "subscription", name: "source" } }],
    };

    expect(renameTransferredResource(source, " imported ")).toEqual({ ...source, name: "imported" });
    expect(copyTransferredResource(source, "copy", new Date("2026-08-15T01:02:03Z"))).toEqual({
      ...source,
      name: "copy",
      created_at: "2026-08-15T01:02:03.000Z",
      updated_at: "2026-08-15T01:02:03.000Z",
    });
    expect(source.name).toBe("source");
    expect(source.inputs[0].ref.name).toBe("source");
  });

  it("builds stable JSON filenames and validates public resource names", () => {
    expect(resourceTransferFilename("subscription", " provider ")).toBe("subscription-provider.json");
    expect(resourceTransferFilename("file", "bad:name?.yaml")).toBe("file-bad_name_.yaml.json");
    expect(resourceNameIssue(" ")).toBe("empty");
    expect(resourceNameIssue("a/b")).toBe("invalid");
    expect(resourceNameIssue("client.yaml")).toBeNull();
  });
});
