import { runInNewContext } from "node:vm";

import { describe, expect, it } from "vitest";

import { createTranslator } from "~/shared/i18n/context";
import type { ProcessorDetail } from "~/shared/resources/types";

import {
  GITHUB_RULE_SOURCE_MIRROR_PRESET_ID,
  GITHUB_RULE_SOURCE_MIRROR_REPLACEMENTS,
  githubRuleSourceMirrorPreset,
  githubRuleSourceMirrorProcessorPreset,
  recognizeGitHubRuleSourceMirrorProcessorPreset,
} from "./github-rule-source-mirror-preset";

describe("GitHub rule source mirror preset", () => {
  it("uses the localized shortcut label as the generated processor name", () => {
    expect(githubRuleSourceMirrorPreset.build(createTranslator("zh-CN")).name)
      .toBe("GitHub 规则源镜像替换");
    expect(githubRuleSourceMirrorPreset.build(createTranslator("en-US")).name)
      .toBe("GitHub rule source mirror replacement");
  });

  it("serializes an editable parameterized inline file script", () => {
    const processor = githubRuleSourceMirrorProcessorPreset("GitHub 规则源镜像替换");
    const content = inlineScriptContent(processor);

    expect(processor).toMatchObject({
      name: "GitHub 规则源镜像替换",
      type: "script",
      stage: "file",
      params: {
        source: { type: "inline" },
        args: {
          preset_id: GITHUB_RULE_SOURCE_MIRROR_PRESET_ID,
          replacements_json: JSON.stringify(GITHUB_RULE_SOURCE_MIRROR_REPLACEMENTS),
        },
      },
    });
    expect(content).toContain("// Parameters:");
    expect(content).toContain("// - preset_id:");
    expect(content).toContain("// - replacements_json:");
    expect(content).not.toContain("raw.githubusercontent.com");
    expect(content).not.toContain("cdn.jsdelivr.net");
  });

  it("rewrites every built-in Raw prefix in order and preserves unrelated content", () => {
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

    expect(runPreset("a", { replacements_json: JSON.stringify([["a", "ab"], ["ab", "done"]]) })).toBe("done");
  });

  it("is idempotent and preserves the rest of the file envelope", () => {
    const input = {
      stage: "file",
      file: {
        name: "config.yaml",
        kind: "mihomo",
        content: "https://cdn.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@meta/geo/geosite/cn.mrs",
        meta: { owner: "user" },
        warnings: [{ code: "existing", message: "keep" }],
      },
      warnings: [{ code: "chain", message: "keep" }],
    };
    const expected = { ...structuredClone(input), args: processorArgs() };

    expect(runEnvelope(input)).toEqual(expected);
    expect(runEnvelope({ stage: "file", warnings: [] })).toEqual({
      stage: "file",
      warnings: [],
      args: processorArgs(),
    });
  });

  it("rejects missing, malformed, and request-overridden managed parameters", () => {
    expect(() => runEnvelope({ file: { content: "a" } }, { replacements_json: "{}" })).toThrow("requires ordered");
    expect(() => runEnvelope({ file: { content: "a" } }, { replacements_json: JSON.stringify([["a", 1]]) })).toThrow("requires ordered");
    expect(() => runEnvelopeWithArgs({ file: { content: "a" } }, { preset_id: GITHUB_RULE_SOURCE_MIRROR_PRESET_ID })).toThrow("replacements_json");
    expect(() => runEnvelope({
      file: { content: "a" },
      request: { args: { replacements_json: "[]" } },
    })).toThrow("cannot be overridden");
  });

  it("recognizes edited new parameters and legacy marker processors", () => {
    const processor = githubRuleSourceMirrorProcessorPreset("GitHub 规则源镜像替换");
    const source = processor.params?.source as Record<string, unknown>;
    const args = processor.params?.args as Record<string, unknown>;

    expect(recognizeGitHubRuleSourceMirrorProcessorPreset(processor)).toBe(true);
    expect(recognizeGitHubRuleSourceMirrorProcessorPreset({
      ...processor,
      params: { source, args: { ...args, replacements_json: "[]" } },
    })).toBe(true);
    expect(recognizeGitHubRuleSourceMirrorProcessorPreset({
      type: "script",
      params: {
        source: {
          type: "inline",
          content: "// sandrone:file-preset=github-rule-source-rewrite\nfunction main(input) { return input; }",
        },
      },
    })).toBe(true);
    expect(recognizeGitHubRuleSourceMirrorProcessorPreset({
      ...processor,
      params: { source: { ...source, content: "function main(input) { return input; }" }, args },
    })).toBe(false);
    expect(recognizeGitHubRuleSourceMirrorProcessorPreset({
      ...processor,
      params: { source: { type: "file", name: "rewrite.js" }, args },
    })).toBe(false);
    expect(recognizeGitHubRuleSourceMirrorProcessorPreset({ ...processor, type: "merge" })).toBe(false);
  });
});

function inlineScriptContent(processor: ProcessorDetail): string {
  const source = processor.params?.source as Record<string, unknown> | undefined;
  if (typeof source?.content !== "string") throw new Error("expected inline script content");
  return source.content;
}

function processorArgs(): Record<string, unknown> {
  const processor = githubRuleSourceMirrorProcessorPreset("GitHub 规则源镜像替换");
  return structuredClone(processor.params?.args as Record<string, unknown>);
}

function runPreset(content: string, argsPatch: Record<string, unknown> = {}): string {
  return runEnvelope({ file: { content } }, argsPatch).file.content;
}

function runEnvelope<T extends Record<string, unknown>>(input: T, argsPatch: Record<string, unknown> = {}): T & { args: Record<string, unknown> } {
  return runEnvelopeWithArgs(input, { ...processorArgs(), ...argsPatch });
}

function runEnvelopeWithArgs<T extends Record<string, unknown>>(input: T, args: Record<string, unknown>): T & { args: Record<string, unknown> } {
  const envelope = { ...input, args };
  const context: {
    api: { json: { parse: (value: string) => unknown } };
    input: typeof envelope;
    output?: typeof envelope;
  } = {
    api: { json: { parse: (value) => JSON.parse(value) as unknown } },
    input: envelope,
  };
  runInNewContext(
    `${inlineScriptContent(githubRuleSourceMirrorProcessorPreset("GitHub 规则源镜像替换"))}\nglobalThis.output = main(input, api);`,
    context,
  );
  if (!context.output) throw new Error("expected script output");
  return context.output;
}
