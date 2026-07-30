import { runInNewContext } from "node:vm";

import { describe, expect, it } from "vitest";

import type { ProcessorDetail } from "~/shared/resources/types";

import {
  recognizeRuleSourceRewriteProcessorPreset,
  ruleSourceRewriteProcessorPreset,
} from "./rule-source-rewrite-preset";

describe("GitHub rule source rewrite preset", () => {
  it("serializes as a neutral editable inline file script", () => {
    const processor = ruleSourceRewriteProcessorPreset();
    const content = inlineScriptContent(processor);

    expect(processor).toMatchObject({
      name: "GitHub Rule Source Rewrite",
      type: "script",
      stage: "file",
      params: { source: { type: "inline" } },
    });
    expect(processor.name).not.toContain("jsDelivr");
    expect(content).toContain("// sandrone:file-preset=github-rule-source-rewrite");
    expect(content).not.toContain("github-rule-source-jsdelivr");
  });

  it("rewrites every built-in Raw prefix and preserves unrelated content", () => {
    const unrelated = "https://raw.githubusercontent.com/example/rules/main/list.txt";
    const input = [
      "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/meta/geo/geosite/cn.mrs",
      "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/meta/geo/geoip/cn.mrs?download=1",
      "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/sing/geo/geosite/cn.srs",
      "https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Shadowrocket/Apple/Apple.list",
      unrelated,
    ].join("\n");

    expect(runPreset(input)).toBe([
      "https://cdn.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@meta/geo/geosite/cn.mrs",
      "https://cdn.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@meta/geo/geoip/cn.mrs?download=1",
      "https://cdn.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@sing/geo/geosite/cn.srs",
      "https://cdn.jsdelivr.net/gh/blackmatrix7/ios_rule_script@master/rule/Shadowrocket/Apple/Apple.list",
      unrelated,
    ].join("\n"));
  });

  it("is idempotent and leaves content without a match unchanged", () => {
    const jsDelivr = "https://cdn.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@meta/geo/geosite/cn.mrs";
    expect(runPreset(jsDelivr)).toBe(jsDelivr);
    expect(runPreset("mode: rule\n")).toBe("mode: rule\n");
  });

  it("preserves the rest of the file envelope and tolerates a missing file", () => {
    const input = {
      stage: "file",
      file: {
        name: "config.yaml",
        kind: "mihomo",
        content: "https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/meta/geo/geosite/cn.mrs",
        meta: { owner: "user" },
        warnings: [{ code: "existing", message: "keep" }],
      },
      warnings: [{ code: "chain", message: "keep" }],
    };
    const expected = structuredClone(input);
    expected.file.content = "https://cdn.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@meta/geo/geosite/cn.mrs";

    expect(runEnvelope(input)).toEqual(expected);
    expect(runEnvelope({ stage: "file", warnings: [] })).toEqual({
      stage: "file",
      warnings: [],
    });
  });

  it("recognizes the neutral marker after users edit the destination", () => {
    const processor = ruleSourceRewriteProcessorPreset();
    const content = inlineScriptContent(processor);
    const customized = {
      ...processor,
      params: {
        source: {
          type: "inline",
          content: content.replaceAll("https://cdn.jsdelivr.net", "https://mirror.example.com"),
        },
      },
    };

    expect(recognizeRuleSourceRewriteProcessorPreset(processor)).toBe(true);
    expect(recognizeRuleSourceRewriteProcessorPreset(customized)).toBe(true);
    expect(recognizeRuleSourceRewriteProcessorPreset({
      ...processor,
      params: { source: { type: "inline", content: "function main(input) { return input; }" } },
    })).toBe(false);
    expect(recognizeRuleSourceRewriteProcessorPreset({
      ...processor,
      params: { source: { type: "file", name: "rewrite.js" } },
    })).toBe(false);
    expect(recognizeRuleSourceRewriteProcessorPreset({
      ...processor,
      type: "merge",
    })).toBe(false);
  });
});

function inlineScriptContent(processor: ProcessorDetail): string {
  const source = processor.params?.source as Record<string, unknown> | undefined;
  if (typeof source?.content !== "string") throw new Error("expected inline script content");
  return source.content;
}

function runPreset(content: string): string {
  return runEnvelope({ file: { content } }).file.content;
}

function runEnvelope<T extends Record<string, unknown>>(input: T): T {
  const context: { input: T; output?: T } = { input };
  runInNewContext(
    `${inlineScriptContent(ruleSourceRewriteProcessorPreset())}\nglobalThis.output = main(input);`,
    context,
  );
  if (!context.output) throw new Error("expected script output");
  return context.output;
}
