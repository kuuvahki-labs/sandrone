import { describe, expect, it, vi } from "vitest";

import type { SubscriptionPreviewNode } from "~/features/subscriptions/model/types";
import type { ApiClient } from "~/shared/api/client";
import { createTranslator } from "~/shared/i18n/context";

import { createNodeToolsActions } from "./create-node-tools-actions";

const node: SubscriptionPreviewNode = {
  endpoint: "proxy.example.com:443",
  name: "fixture-node",
  port: 443,
  raw: {
    name: "fixture-node",
    password: "fixture-password",
    port: 443,
    server: "proxy.example.com",
    type: "trojan",
  },
  server: "proxy.example.com",
  type: "trojan",
};

describe("createNodeToolsActions", () => {
  it("requests URI information for exactly the selected final NodeIR", async () => {
    const inspectNode = vi.fn().mockResolvedValue({
      uri: { value: "trojan://fixture-password@proxy.example.com:443#fixture-node", warnings: [] },
    });
    const actions = createNodeToolsActions({
      client: { inspectNode } as unknown as ApiClient,
      showNotice: vi.fn(),
      t: createTranslator("zh-CN"),
    });

    const result = await actions.renderNodeURI(node);

    expect(inspectNode).toHaveBeenCalledWith({ node: node.raw, include: ["uri"] });
    expect(result).toEqual({
      uri: "trojan://fixture-password@proxy.example.com:443#fixture-node",
      warnings: [],
    });
  });

  it("preserves URI rendering warnings", async () => {
    const actions = createNodeToolsActions({
      client: {
        inspectNode: vi.fn().mockResolvedValue({
          uri: {
            value: "trojan://fixture-password@proxy.example.com:443#fixture-node",
            warnings: [{ code: "render_lossy_field", field: "flow", message: "field omitted", node: "fixture-node" }],
          },
        }),
      } as unknown as ApiClient,
      showNotice: vi.fn(),
      t: createTranslator("zh-CN"),
    });

    const result = await actions.renderNodeURI(node);

    expect(result.warnings).toEqual([expect.objectContaining({ code: "render_lossy_field", field: "flow" })]);
  });

  it("rejects unsupported or ambiguous renderer output", async () => {
    const actions = createNodeToolsActions({
      client: {
        inspectNode: vi.fn().mockResolvedValue({ uri: { value: "ss://one\nss://two", warnings: [] } }),
      } as unknown as ApiClient,
      showNotice: vi.fn(),
      t: createTranslator("zh-CN"),
    });

    await expect(actions.renderNodeURI(node)).rejects.toThrow("当前节点无法生成可用的 URI。");
  });

  it("requests IP information for the selected final NodeIR", async () => {
    const inspectNode = vi.fn().mockResolvedValue({
      ip: {
        server: "proxy.example.com",
        ip: "198.18.0.1",
        ip_version: 4,
        public: false,
      },
    });
    const actions = createNodeToolsActions({
      client: { inspectNode } as unknown as ApiClient,
      showNotice: vi.fn(),
      t: createTranslator("zh-CN"),
    });

    await expect(actions.lookupNodeIPInfo(node)).resolves.toEqual({
      server: "proxy.example.com",
      ip: "198.18.0.1",
      ipVersion: 4,
      public: false,
      countryCode: undefined,
      country: undefined,
      continentCode: undefined,
      continent: undefined,
      asn: undefined,
      asName: undefined,
      asDomain: undefined,
      source: undefined,
    });
    expect(inspectNode).toHaveBeenCalledWith({ node: node.raw, include: ["ip"] });
  });
});
