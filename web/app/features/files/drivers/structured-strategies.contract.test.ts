import { readFileSync } from "node:fs";

import { describe, expect, it } from "vitest";

import type {
  AdaptiveGroupGeneration,
  AdaptiveGroupMergeResult,
  AdaptiveGroupOptions,
  ConfigAdaptiveDialect,
} from "~/features/files/config/model/adaptive-groups";
import {
  adaptiveGroupHelpers,
  defaultAdaptiveGroupOptions,
} from "~/features/files/config/model/adaptive-groups";
import {
  configNodePreviewFromSubscription,
  type ConfigNodePreviewInput,
} from "~/features/files/config/model/node-source";
import type { ConfigRelationProjection } from "~/features/files/config/model/relations";
import { buildConfigRelationModel } from "~/features/files/config/model/relations";
import type { ConfigTemplateID, ConfigTemplateRecognition } from "~/features/files/config/model/templates";
import type { FileConfigDraft } from "~/features/files/model/types";

import type { StructuredFileConfigurationAdapter } from "./core/file-driver";
import { requireFileDriver } from "./registry";

const CONFIG_KINDS = ["mihomo", "sing-box", "shadowrocket"] as const;
const TEMPLATE_IDS = ["minimal", "standard", "full"] as const satisfies readonly ConfigTemplateID[];
const NAMING_LOCALES = ["en-US", "zh-CN"] as const;

interface Task3AdapterStrategies {
  adaptive: {
    canonicalNames: (groups: readonly Record<string, unknown>[]) => string[];
    generate: (nodeNames: readonly string[], options: Readonly<AdaptiveGroupOptions>, namingLocale?: "en-US" | "zh-CN") => AdaptiveGroupGeneration;
    merge: (config: Readonly<FileConfigDraft>, generation: Readonly<AdaptiveGroupGeneration>) => AdaptiveGroupMergeResult;
 };
  preview: {
    projectNodes: (preview: ReturnType<typeof configNodePreviewFromSubscription>) => Array<{ name: string }>;
 };
  relations: {
    project: (groups: Record<string, unknown>[], ruleSets: Record<string, unknown>[], rules: unknown[], nodeNames?: string[]) => ConfigRelationProjection;
 };
  templates: {
    create: (id: ConfigTemplateID, namingLocale?: "en-US" | "zh-CN") => FileConfigDraft;
    list: () => Array<{ id: ConfigTemplateID; modules: string[] }>;
    recognize: (config: FileConfigDraft) => ConfigTemplateRecognition;
 };
}

describe("structured file driver orchestration strategies", () => {
  it.each(CONFIG_KINDS)("registers immutable Task 3 strategies for %s", (kind) => {
    const adapter = task3Adapter(kind);

    expect(adapter.templates).toEqual(expect.objectContaining({
      create: expect.any(Function),
      list: expect.any(Function),
      recognize: expect.any(Function),
   }));
    expect(adapter.relations).toEqual(expect.objectContaining({ project: expect.any(Function) }));
    expect(adapter.preview).toEqual(expect.objectContaining({ projectNodes: expect.any(Function) }));
    expect(adapter.adaptive).toEqual(expect.objectContaining({
      canonicalNames: expect.any(Function),
      generate: expect.any(Function),
      merge: expect.any(Function),
   }));
    expect(Object.isFrozen(adapter.templates)).toBe(true);
    expect(Object.isFrozen(adapter.relations)).toBe(true);
    expect(Object.isFrozen(adapter.preview)).toBe(true);
    expect(Object.isFrozen(adapter.adaptive)).toBe(true);
 });

  it.each(CONFIG_KINDS)("keeps every %s template tier recognizable in both naming locales", (kind) => {
    const adapter = task3Adapter(kind);

    expect(adapter.templates.list().map((template) => template.id)).toEqual(TEMPLATE_IDS);
    for (const namingLocale of NAMING_LOCALES) {
      for (const templateID of TEMPLATE_IDS) {
        const config = adapter.templates.create(templateID, namingLocale);
        expect(adapter.templates.recognize(config)).toEqual({
          adaptive: false,
          match: templateID,
          namingLocale,
       });
        expect(buildConfigRelationModel(adapter.relations.project(
          config.groups ?? [],
          config.rule_sets ?? [],
          config.rules ?? [],
        )).issues).toEqual([]);
     }
   }
 });

  it.each(CONFIG_KINDS)("owns %s native adaptive metadata cleanup during template recognition", (kind) => {
    const adapter = task3Adapter(kind);
    const config = adapter.templates.create("minimal", "en-US");

    expect(adapter.templates.recognize({
      ...config,
      adaptive_groups: { type: "driver-native" },
   })).toEqual({
      adaptive: false,
      match: "minimal",
      namingLocale: "en-US",
   });
 });

  it("keeps Shadowrocket configuration independent from subscription node previews", () => {
    const adapter = task3Adapter("shadowrocket");
    const preview: ConfigNodePreviewInput = {
      subscriptionName: "provider",
      nodes: [
        { runtimeId: "first", after: { name: "raw node", type: "http", endpoint: "one.example:2" }, targetNames: { shadowrocket: "Rendered Node" } },
      ],
      warnings: [],
   };

    const normalized = configNodePreviewFromSubscription(preview);
    const projected = adapter.preview.projectNodes(normalized);

    expect(projected).toEqual([]);
    expect(adapter.preview.relationNodeNames(projected, true)).toEqual([]);
    expect(adapter.preview.validate({ formMode: "edit", preview: normalized, projectedNodes: projected, selected: true }))
      .toEqual({ valid: true });
 });

  it.each([
    { name: "unique named node", names: ["Node 1"], selected: true, expectedValid: true },
    { name: "duplicate node tags", names: ["Node 1", "Node 1"], selected: true, expectedValid: false },
    { name: "unnamed node", names: ["Node 1", ""], selected: true, expectedValid: false },
    { name: "empty preview", names: [], selected: true, expectedValid: false },
    { name: "unselected subscription", names: ["Node 1"], selected: false, expectedValid: false },
  ])("uses conservative sing-box readiness for $name", ({ names, selected, expectedValid }) => {
    const adapter = task3Adapter("sing-box");
    const preview = configNodePreviewFromSubscription({
      subscriptionName: "provider",
      nodes: names.map((name, index) => ({
        runtimeId: `runtime-${index}`,
        after: { name, type: "ss", endpoint: `node-${index}.example:8388` },
      })),
      warnings: [],
    });
    const projectedNodes = adapter.preview.projectNodes(preview);

    expect(adapter.preview.validate({
      formMode: "create",
      preview,
      projectedNodes,
      selected,
    }).valid).toBe(expectedValid);
  });

  it("keeps registered client selection and native keys out of shared Task 3 orchestration", () => {
    const sharedFiles = [
      "../config/components/editor.tsx",
      "../config/model/editor-state.ts",
      "../config/model/references.ts",
      "../config/model/templates.ts",
      "../config/model/relations.ts",
      "../config/model/adaptive-groups.ts",
      "../config/model/adaptive-availability.ts",
    ];

    for (const filename of sharedFiles) {
      const source = readFileSync(new URL(filename, import.meta.url), "utf8");
      expect(source, filename).not.toMatch(/["'](?:mihomo|sing-box|shadowrocket)["']/);
      expect(source, filename).not.toMatch(/["'](?:tag|outbounds|proxies|policy-regex-filter)["']/);
      expect(source, filename).not.toMatch(/adaptive_groups/);
      expect(source, filename).not.toMatch(/persistOptions|staleMode|createConfigAdaptiveStrategy/);
   }

    const previewSource = readFileSync(new URL("../config/model/node-source.ts", import.meta.url), "utf8");
    expect(previewSource).not.toContain("normalizeShadowrocketNodeName");
    expect(previewSource).not.toContain("SHADOWROCKET_SUPPORTED_NODE_TYPES");
 });

  it("lets a fourth driver compose custom adaptive persistence and stale behavior from pure helpers", () => {
    const dialect = fakeAdaptiveDialect();
    const custom = {
      ...adaptiveGroupHelpers(dialect),
      configFromOptions: (options: Readonly<AdaptiveGroupOptions>) => ({ type: `fourth:${options.type}` }),
      initiallyEnabled: (_mode: "create" | "edit", config: FileConfigDraft["adaptive_groups"]) => config?.type?.startsWith("fourth:") === true,
      isStale: ({ options }: { options: Readonly<AdaptiveGroupOptions> }) => options.type === "stale-by-driver",
      optionsFromConfig: (config: FileConfigDraft["adaptive_groups"]) => ({
        ...defaultAdaptiveGroupOptions(dialect),
        type: config?.type?.replace(/^fourth:/, "") ?? "race",
     }),
      recognizesCanonicalLayer: () => false,
   } satisfies Task3AdapterStrategies["adaptive"] & {
      configFromOptions: (options: Readonly<AdaptiveGroupOptions>) => FileConfigDraft["adaptive_groups"];
      initiallyEnabled: (mode: "create" | "edit", config: FileConfigDraft["adaptive_groups"]) => boolean;
      isStale: (input: { options: Readonly<AdaptiveGroupOptions> }) => boolean;
      optionsFromConfig: (config: FileConfigDraft["adaptive_groups"]) => AdaptiveGroupOptions;
      recognizesCanonicalLayer: (config: Readonly<FileConfigDraft>) => boolean;
   };

    const persisted = custom.configFromOptions({ type: "race" });
    expect(persisted).toEqual({ type: "fourth:race" });
    expect(custom.initiallyEnabled("edit", persisted)).toBe(true);
    expect(custom.optionsFromConfig(persisted).type).toBe("race");
    expect(custom.isStale({ options: { type: "stale-by-driver" } })).toBe(true);
    expect(custom.generate(["HK-01"], { type: "race" }).groups[0]).toMatchObject({
      label: "Hong Kong",
      members: ["HK-01"],
   });
 });
});

function fakeAdaptiveDialect(): ConfigAdaptiveDialect {
  return {
    anchorProblem: () => null,
    canonicalName: () => undefined,
    defaultType: "race",
    groupMembers: (group) => Array.isArray(group.members) ? group.members.map(String) : [],
    groupName: (group) => typeof group.label === "string" ? group.label : "",
    inboundReferences: () => ({}),
    materialize: (definition, _type, nodeNames) => ({ label: definition.name, members: [...nodeNames] }),
    replaceGroupMembers: (group, members) => ({ ...group, members: [...members] }),
    requiresNodePreview: true,
    typeOptions: [{ label: "race", value: "race" }, { label: "stale", value: "stale-by-driver" }],
 };
}

function task3Adapter(kind: typeof CONFIG_KINDS[number]): StructuredFileConfigurationAdapter & Task3AdapterStrategies {
  const driver = requireFileDriver(kind);
  if (driver.configuration.mode !== "structured") throw new Error(`${kind} must be structured`);
  return driver.configuration.adapter as StructuredFileConfigurationAdapter & Task3AdapterStrategies;
}
