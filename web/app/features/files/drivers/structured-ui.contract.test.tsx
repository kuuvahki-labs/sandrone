import { useState } from "react";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, expectTypeOf, it, vi } from "vitest";

import { ProxyGroupEditor } from "~/features/files/config/components/group-editor";
import { RuleListEditor, RuleSetListEditor } from "~/features/files/config/components/rule-editor";
import type { GroupDraft, RuleDraft, RuleSetDraft } from "~/features/files/config/model/editor-model";
import type {
  GroupFieldsProps,
  RuleFieldsProps,
  RuleSetFieldsProps,
  StructuredConfigurationFieldSlots,
} from "~/features/files/editor/file-driver-ui";
import {
  FILE_DRIVER_UI_REGISTRY,
  fileDriverUI,
  requireFileDriverUI,
} from "~/features/files/editor/file-driver-ui-registry";

import type {
  StructuredFileConfigurationAdapter,
} from "./core/file-driver";
import { mihomoConfigurationFields } from "./mihomo/fields";
import { FILE_DRIVER_REGISTRY, requireFileDriver } from "./registry";
import { shadowrocketConfigurationFields } from "./shadowrocket/fields";
import { singBoxConfigurationFields } from "./sing-box/fields";

const inheritedRegistryKeys = ["toString", "constructor", "__proto__"] as const;

describe("structured driver UI slots", () => {
  it("exposes semantic-only slot props instead of structured codec capabilities", () => {
    expectTypeOf<GroupFieldsProps>().not.toHaveProperty("capabilities");
    expectTypeOf<GroupFieldsProps["draft"]>().not.toHaveProperty("adapterState");
    expectTypeOf<RuleSetFieldsProps>().not.toHaveProperty("capabilities");
    expectTypeOf<RuleFieldsProps>().not.toHaveProperty("capabilities");
  });

  it("registers exactly the structured driver keys and no raw-only entry", () => {
    const structuredKinds = FILE_DRIVER_REGISTRY.drivers
      .filter((driver) => driver.configuration.mode === "structured")
      .map((driver) => driver.kind);

    expect(Object.keys(FILE_DRIVER_UI_REGISTRY)).toEqual(structuredKinds);
    expect(FILE_DRIVER_UI_REGISTRY).not.toHaveProperty("static");
    expect(fileDriverUI("static")).toBeUndefined();
    expect(fileDriverUI(undefined)).toBeUndefined();
    expect(Object.isFrozen(FILE_DRIVER_UI_REGISTRY)).toBe(true);
  });

  it.each(inheritedRegistryKeys)("returns undefined for inherited registry key %s", (kind) => {
    expect(fileDriverUI(kind)).toBeUndefined();
  });

  it.each(inheritedRegistryKeys)("throws the exact missing-UI error for inherited registry key %s", (kind) => {
    expect(errorMessage(() => requireFileDriverUI(kind)))
      .toBe(`unregistered file driver UI: ${kind}`);
  });

  it.each([
    ["mihomo", mihomoConfigurationFields, {
      headerLayout: "name-fields-source",
      intervalInputType: "number",
      remoteFields: "format-interval",
      sourceMode: "switchable",
      summaryFields: ["behavior", "format"],
    }],
    ["sing-box", singBoxConfigurationFields, {
      headerLayout: "name-source",
      intervalInputType: "text",
      remoteFields: "format-interval",
      sourceMode: "switchable",
      summaryFields: ["format"],
    }],
    ["shadowrocket", shadowrocketConfigurationFields, {
      headerLayout: "name-fields",
      intervalInputType: "text",
      remoteFields: "url-only",
      sourceMode: "remote-only",
      summaryFields: ["behavior"],
    }],
  ] as const)("preserves frozen %s field identity and presentation", (kind, concreteFields, presentation) => {
      const ui = requireFileDriverUI(kind);

      expect(ui).toEqual({
        GroupFields: expect.any(Function),
        RuleFields: expect.any(Function),
        RuleSetFields: expect.any(Function),
        ruleSetPresentation: presentation,
      });
      expect(ui.GroupFields).toBe(concreteFields.GroupFields);
      expect(ui.RuleFields).toBe(concreteFields.RuleFields);
      expect(ui.RuleSetFields).toBe(concreteFields.RuleSetFields);
      expect(fileDriverUI(kind)).toBe(FILE_DRIVER_UI_REGISTRY[kind]);
      expect(Object.isFrozen(ui)).toBe(true);
      expect(Object.isFrozen(ui.ruleSetPresentation)).toBe(true);
      expect(Object.isFrozen(ui.ruleSetPresentation.summaryFields)).toBe(true);
    });

  it("lets a driver slot update only a normalized group draft", async () => {
    const user = userEvent.setup();
    const adapter = structuredAdapter("shadowrocket");
    const onUpdate = vi.fn();
    const draft = groupDraft({
      healthCheckInterval: "300",
      healthCheckTimeout: 5,
      healthCheckTolerance: 50,
      name: "Auto",
      selectedIndex: 1,
      type: "url-test",
    });
    const GroupFields = requireFileDriverUI("shadowrocket").GroupFields;

    render(
      <GroupFields
        draft={draft}
        healthCheck={adapter.groups.isHealthCheck(draft.type)}
        index={0}
        onUpdate={onUpdate}
      />,
    );

    const select = screen.getByRole("spinbutton", { name: /默认策略索引|Default policy index/i });
    await user.clear(select);

    expect(onUpdate).toHaveBeenLastCalledWith({ ...draft, selectedIndex: undefined });
    expect(onUpdate.mock.lastCall?.[0]).not.toHaveProperty("select");
  });

  it.each([
    ["mihomo", {
      name: "Proxy",
      type: "select",
      proxies: ["$nodes", "DIRECT"],
      icon: "https://example.com/icon.png",
      future_nested: { keep: true },
    }, {
      name: "Edited Proxy",
      type: "select",
      proxies: ["$nodes", "DIRECT"],
      icon: "https://example.com/icon.png",
      future_nested: { keep: true },
    }],
    ["sing-box", {
      type: "selector",
      tag: "Proxy",
      outbounds: ["$nodes", "direct"],
      interrupt_exist_connections: true,
      future_nested: { keep: true },
    }, {
      type: "selector",
      tag: "Edited Proxy",
      outbounds: ["$nodes", "direct"],
      interrupt_exist_connections: true,
      future_nested: { keep: true },
    }],
  ] as const)("keeps opaque %s group state outside the UI slot and lossless after its update", async (
    kind,
    nativeGroup,
    expectedNativeGroup,
  ) => {
    const user = userEvent.setup();
    const base = structuredAdapter(kind);
    const initial = base.decode({
      settingsPresent: true,
      settings: { groups: [nativeGroup], rule_sets: [], rules: [] },
    }, "en-US")!;
    let latestGroups = initial.groups;

    function ProbeGroupFields(props: GroupFieldsProps) {
      const runtimeProps = props as unknown as Record<string, unknown>;
      const runtimeDraft = props.draft as unknown as Record<string, unknown>;
      const injectedPatch = {
        ...props.draft,
        adapterState: { injected: true },
        name: "Edited Proxy",
      } as unknown as Parameters<GroupFieldsProps["onUpdate"]>[0];
      return (
        <button
          data-has-adapter-state={String(Object.hasOwn(runtimeDraft, "adapterState"))}
          data-has-capabilities={String(Object.hasOwn(runtimeProps, "capabilities"))}
          type="button"
          onClick={() => props.onUpdate(injectedPatch)}
        >
          update semantic group
        </button>
      );
    }

    const ui: StructuredConfigurationFieldSlots = {
      ...requireFileDriverUI(kind),
      GroupFields: ProbeGroupFields,
    };
    render(
      <OpaqueGroupHarness
        adapter={base}
        groups={initial.groups}
        ui={ui}
        onChange={(groups) => { latestGroups = groups; }}
      />,
    );

    await user.click(screen.getByRole("button", { name: "展开代理组 Proxy" }));

    const updateButton = screen.getByRole("button", { name: "update semantic group" });
    expect(updateButton).toHaveAttribute("data-has-capabilities", "false");
    expect(updateButton).toHaveAttribute("data-has-adapter-state", "false");

    await user.click(updateButton);

    expect(base.encode({ ...initial, groups: latestGroups })).toEqual({
      settings: { groups: [expectedNativeGroup], rule_sets: [], rules: [] },
    });
  });

  it("renders a fake fourth structured driver's slots without shared-code changes", async () => {
    const user = userEvent.setup();
    const adapter = futureAdapter();
    const ui = futureUI();
    expect(adapter).not.toHaveProperty("ui");
    render(<FutureDriverHarness adapter={adapter} ui={ui} />);

    await user.click(screen.getByRole("button", { name: "展开代理组 Future group" }));
    expect(screen.getByText("future group fields: Future group")).toBeInTheDocument();

    const ruleSetRow = screen.getByRole("group", { name: "规则集 1 future-set" });
    expect(within(ruleSetRow).getByText("future-only")).toBeInTheDocument();
    expect(within(ruleSetRow).getByText("future-json")).toBeInTheDocument();

    await user.click(within(ruleSetRow).getByRole("button", { name: "展开规则集 1 future-set" }));

    expect(within(ruleSetRow).getByText("future rule-set fields: future-set")).toBeInTheDocument();
    expect(within(ruleSetRow).queryByRole("group", { name: /来源|Source/i })).not.toBeInTheDocument();
    expect(within(ruleSetRow).getByRole("combobox", { name: /格式|Format/i })).toHaveTextContent("future-json");
    expect(within(ruleSetRow).getByRole("textbox", { name: /更新间隔|Update interval/i })).toHaveValue("15m");
    const nameField = within(ruleSetRow).getByRole("textbox", { name: /^名称$|^Name$/i });
    expect(nameField.closest(".grid")).toHaveClass("md:grid-cols-2");

    await user.click(screen.getByRole("button", { name: "展开规则 1" }));
    expect(screen.getByText("future rule fields: custom-rule")).toBeInTheDocument();
  });
});

function OpaqueGroupHarness({ adapter, groups: initialGroups, onChange, ui }: {
  adapter: StructuredFileConfigurationAdapter;
  groups: GroupDraft[];
  onChange: (groups: GroupDraft[]) => void;
  ui: StructuredConfigurationFieldSlots;
}) {
  const [groups, setGroups] = useState(initialGroups);
  return (
    <ProxyGroupEditor
      adapter={adapter}
      groups={groups}
      inboundReferences={{}}
      issues={[]}
      nodes={[]}
      ui={ui}
      onChange={(next) => {
        setGroups(next);
        onChange(next);
      }}
    />
  );
}

function FutureDriverHarness({ adapter, ui }: {
  adapter: StructuredFileConfigurationAdapter;
  ui: StructuredConfigurationFieldSlots;
}) {
  const [groups, setGroups] = useState([groupDraft({ name: "Future group" })]);
  const [ruleSets, setRuleSets] = useState([ruleSetDraft({
    behavior: "future-only",
    format: "future-json",
    interval: "15m",
    source: "remote",
    url: "https://example.com/future.json",
  })]);
  const [rules, setRules] = useState([ruleDraft()]);
  return (
    <>
      <ProxyGroupEditor
        adapter={adapter}
        groups={groups}
        inboundReferences={{}}
        issues={[]}
        nodes={[]}
        ui={ui}
        onChange={setGroups}
      />
      <RuleSetListEditor
        adapter={adapter}
        inboundReferences={{}}
        issues={[]}
        ruleSets={ruleSets}
        ui={ui}
        onChange={setRuleSets}
      />
      <RuleListEditor
        adapter={adapter}
        defaultExpanded
        groups={groups}
        issues={[]}
        nodes={[]}
        rules={rules}
        ruleSets={ruleSets}
        ui={ui}
        onChange={setRules}
      />
    </>
  );
}

function FutureGroupFields({ draft }: GroupFieldsProps) {
  return <p>future group fields: {draft.name}</p>;
}

function FutureRuleSetFields({ draft }: RuleSetFieldsProps) {
  return <p>future rule-set fields: {draft.name}</p>;
}

function FutureRuleFields({ draft }: RuleFieldsProps) {
  return <p>future rule fields: {draft.value}</p>;
}

function futureAdapter(): StructuredFileConfigurationAdapter {
  const base = structuredAdapter("mihomo");
  return {
    ...base,
    kind: "future-client",
    ruleSets: {
      behaviorOptions: () => [{ value: "future-only", label: "future-only" }],
      create: () => ruleSetDraft({
        behavior: "future-only",
        format: "future-json",
        interval: "15m",
        source: "remote",
        url: "https://example.com/future.json",
      }),
      formatOptions: [{ value: "future-json", label: "future-json" }],
      formatPatch: (url, format) => ({ format, url }),
      fromCatalog: () => ({ status: "name-conflict", existingName: "future-set" }),
      project: () => [],
      serialize: () => [],
    },
  };
}

function futureUI(): StructuredConfigurationFieldSlots {
  return {
    GroupFields: FutureGroupFields,
    RuleFields: FutureRuleFields,
    RuleSetFields: FutureRuleSetFields,
    ruleSetPresentation: {
      headerLayout: "name-fields",
      intervalInputType: "text",
      remoteFields: "format-interval",
      sourceMode: "remote-only",
      summaryFields: ["behavior", "format"],
    },
  };
}

function structuredAdapter(kind: string): StructuredFileConfigurationAdapter {
  const configuration = requireFileDriver(kind).configuration;
  if (configuration.mode !== "structured") throw new Error(`expected structured driver: ${kind}`);
  return configuration.adapter;
}

function errorMessage(action: () => unknown): string | undefined {
  try {
    action();
  } catch (error) {
    expect(error).toBeInstanceOf(Error);
    return (error as Error).message;
  }
  return undefined;
}

function groupDraft(overrides: Partial<GroupDraft> = {}): GroupDraft {
  return {
    excludeFilter: "",
    filter: "",
    healthCheckInterval: "",
    healthCheckURL: "",
    id: "group-future",
    memberMode: "fixed",
    members: ["$nodes"],
    name: "Future group",
    type: "select",
    ...overrides,
  };
}

function ruleSetDraft(overrides: Partial<RuleSetDraft> = {}): RuleSetDraft {
  return {
    behavior: "classical",
    format: "yaml",
    id: "ruleset-future",
    interval: "86400",
    name: "future-set",
    payloadText: "DOMAIN-SUFFIX,example.com",
    source: "inline",
    url: "",
    ...overrides,
  };
}

function ruleDraft(): RuleDraft {
  return {
    id: "rule-future",
    policy: "Future group",
    type: "rule-set",
    value: "custom-rule",
  };
}
