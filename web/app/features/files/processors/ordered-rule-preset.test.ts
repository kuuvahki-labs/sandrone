import { runInNewContext } from "node:vm";

import { describe, expect, it } from "vitest";

import type { ProcessorDetail } from "~/shared/resources/types";

import {
  orderedRuleProcessorPreset as buildOrderedRuleProcessorPreset,
  type OrderedRuleProcessorPresetOptions,
  recognizeOrderedRuleProcessorPreset,
} from "./ordered-rule-preset";

const CASES: readonly OrderedRuleProcessorPresetOptions[] = [
  {
    id: "quic-fallback",
    kind: "mihomo",
    rules: ["AND,((NETWORK,UDP),(DST-PORT,443)),REJECT"],
  },
  {
    id: "quic-fallback",
    kind: "sing-box",
    rules: [{ protocol: "quic", action: "reject" }],
  },
];

const SHADOWROCKET_TOP_OPTIONS: OrderedRuleProcessorPresetOptions = {
  id: "tailscale-native",
  kind: "shadowrocket",
  insertMode: "top",
  rules: ["IP-CIDR,100.64.0.0/10,TAILSCALE,no-resolve"],
};

const LOCALIZED_NAME = "QUIC 强制回退";

describe("ordered rule processor presets", () => {
  it.each(CASES)("documents the managed $kind parameters in the script header", (options) => {
    const header = inlineSource(orderedRuleProcessorPreset(options)).split("function main")[0];

    expect(header).toContain("// Parameters:");
    expect(header).toContain("// - preset_id:");
    expect(header).toContain("// - rules_json:");
    expect(header).not.toContain("insert_mode");
  });

  it.each(CASES)("builds the exact editable $kind inline script params", (options) => {
    const processor = orderedRuleProcessorPreset(options);

    expect(processor).toEqual({
      name: LOCALIZED_NAME,
      type: "script",
      stage: "file",
      params: {
        source: {
          type: "inline",
          content: expect.any(String),
        },
        args: {
          preset_id: options.id,
          rules_json: JSON.stringify(options.rules),
        },
      },
    });
    expect(inlineSource(processor)).toContain(`safe ${options.kind} rule anchor`);
    expect(recognizeOrderedRuleProcessorPreset(processor, options)).toBe(true);
  });

  it.each(CASES)("requires exact source and args when recognizing $kind", (options) => {
    const processor = orderedRuleProcessorPreset(options);
    const params = processor.params as Record<string, unknown>;
    const source = params.source as Record<string, unknown>;
    const args = params.args as Record<string, unknown>;

    expect(recognizeOrderedRuleProcessorPreset({
      ...processor,
      params: { ...params, source: { ...source, content: `${String(source.content)}\n// user edit` } },
    }, options)).toBe(false);
    expect(recognizeOrderedRuleProcessorPreset({
      ...processor,
      params: { ...params, args: { ...args, preset_id: "edited" } },
    }, options)).toBe(false);
    expect(recognizeOrderedRuleProcessorPreset({
      ...processor,
      params: { ...params, args: { ...args, rules_json: "[]" } },
    }, options)).toBe(false);
  });

  it("prefers Mihomo private rule sets, then CN GeoIP, then MATCH", () => {
    const rules = [
      "DOMAIN,user.example,DIRECT",
      "MATCH,Proxy",
      "GEOIP,CN,DIRECT",
      "RULE-SET,private,DIRECT",
    ];

    const output = runPreset(CASES[0], "rules:\n" + rules.map((rule) => `  - ${rule}`).join("\n") + "\n", yamlAPI());

    expect(output).toContain([
      "  - DOMAIN,user.example,DIRECT",
      "  - MATCH,Proxy",
      "  - GEOIP,CN,DIRECT",
      "  - AND,((NETWORK,UDP),(DST-PORT,443)),REJECT",
      "  - RULE-SET,private,DIRECT",
    ].join("\n"));
  });

  it("accepts a sing-box match-all route action and ignores non-final route actions", () => {
    const content = JSON.stringify({
      route: {
        rules: [
          { domain_suffix: ["user.example"], outbound: "direct" },
          { action: "route", outbound: "service", network: "tcp" },
          { action: "route", outbound: "Proxy" },
        ],
      },
    });

    const output = JSON.parse(runPreset(CASES[1], content, jsonAPI())) as {
      route: { rules: unknown[] };
    };

    expect(output.route.rules).toEqual([
      { domain_suffix: ["user.example"], outbound: "direct" },
      { action: "route", outbound: "service", network: "tcp" },
      { protocol: "quic", action: "reject" },
      { action: "route", outbound: "Proxy" },
    ]);
  });

  it("supports top insertion for Shadowrocket rules that must beat remote LAN rule sets", () => {
    const options = SHADOWROCKET_TOP_OPTIONS;
    const document = {
      bom: false,
      newline: "\n",
      trailing_newline: true,
      preamble: [],
      sections: [
        {
          name: "Rule",
          lines: [
            "RULE-SET,https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Shadowrocket/Lan/Lan.list,DIRECT",
            "FINAL,Proxy",
          ],
        },
      ],
    };

    const output = JSON.parse(runPreset(options, JSON.stringify(document), iniModelAPI())) as typeof document;

    expect(output.sections[0].lines).toEqual([
      "IP-CIDR,100.64.0.0/10,TAILSCALE,no-resolve",
      ...document.sections[0].lines,
    ]);
    expect(inlineSource(orderedRuleProcessorPreset(options)).split("function main")[0])
      .toContain("// - insert_mode:");
    expect(recognizeOrderedRuleProcessorPreset(orderedRuleProcessorPreset(options), options)).toBe(true);
  });

  it("recognizes the legacy top-mode Tailscale processor", () => {
    const options = SHADOWROCKET_TOP_OPTIONS;
    const processor = orderedRuleProcessorPreset(options);
    const params = processor.params as { args: Record<string, string>; source: Record<string, string> };
    const legacyProcessor = {
      ...processor,
      params: {
        ...params,
        args: {
          preset_id: params.args.preset_id,
          rules_json: params.args.rules_json,
        },
      },
    };

    expect(recognizeOrderedRuleProcessorPreset(legacyProcessor, options)).toBe(true);
  });
});

function inlineSource(processor: ProcessorDetail): string {
  const source = processor.params?.source as Record<string, unknown> | undefined;
  if (typeof source?.content !== "string") throw new Error("expected inline script content");
  return source.content;
}

function orderedRuleProcessorPreset(options: OrderedRuleProcessorPresetOptions): ProcessorDetail {
  return buildOrderedRuleProcessorPreset(options, LOCALIZED_NAME);
}

function runPreset(
  options: OrderedRuleProcessorPresetOptions,
  content: string,
  api: Record<string, unknown>,
): string {
  const processor = orderedRuleProcessorPreset(options);
  const params = processor.params as {
    args: Record<string, string>;
  };
  const input = { file: { content }, args: params.args };
  const context: { api: Record<string, unknown>; input: typeof input; output?: typeof input } = {
    api,
    input,
  };
  runInNewContext(`${inlineSource(processor)}\nglobalThis.output = main(input, api);`, context);
  if (!context.output) throw new Error("expected script output");
  return context.output.file.content;
}

function jsonAPI(): Record<string, unknown> {
  return {
    json: {
      parse: JSON.parse,
      stringify: JSON.stringify,
    },
  };
}

function yamlAPI(): Record<string, unknown> {
  return {
    ...jsonAPI(),
    yaml: {
      parse: (content: string) => ({ rules: content.trim().split("\n").slice(1).map((line) => line.slice(4)) }),
      stringify: (document: { rules: string[] }) => `rules:\n${document.rules.map((rule) => `  - ${rule}`).join("\n")}\n`,
    },
  };
}

function iniModelAPI(): Record<string, unknown> {
  return {
    ...jsonAPI(),
    ini: {
      parse: JSON.parse,
      stringify: JSON.stringify,
    },
  };
}
