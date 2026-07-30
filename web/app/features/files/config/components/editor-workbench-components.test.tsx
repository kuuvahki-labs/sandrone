import { useState } from "react";
import { act, fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { ConfigReferenceOption } from "~/features/files/config/model/references";
import type { RuleSetCatalogItem, RuleSetCatalogResult } from "~/features/files/model/types";

import { ConfigRowSummary, RowOrderActions, SectionIssues } from "./editor-shared";
import { OrderedReferenceList } from "./reference-fields";
import { RuleSetCatalogDialog } from "./rule-set-catalog-dialog";
import {
  ConfigTemplateAppliedNotice,
  type ConfigTemplateChoice,
  ConfigTemplatePicker,
} from "./template-picker";
import { ConfigWorkbenchSection } from "./workbench-section";

afterEach(() => {
  localStorage.removeItem("sandrone.locale");
  vi.restoreAllMocks();
});

describe("shared config editor rows", () => {
  it("renders long summaries as wrapping plain text", () => {
    const { container } = render(
      <ConfigRowSummary
        primary="Developer Tools With A Very Long Name"
        secondary={["select", "Auto", "订阅节点", "DIRECT"]}
      />,
    );

    expect(container.querySelector(".MuiChip-root")).not.toBeInTheDocument();
    expect(screen.getByText("Developer Tools With A Very Long Name")).toHaveClass("break-words", "whitespace-normal");
    expect(container).toHaveTextContent("select · Auto · 订阅节点 · DIRECT");
  });

  it("keeps desktop controls and mobile menu callbacks accessible", async () => {
    const user = userEvent.setup();
    const onDelete = vi.fn();
    const onDown = vi.fn();
    const onUp = vi.fn();
    render(
      <RowOrderActions
        deleteLabel="删除代理组 Proxy"
        downDisabled={false}
        downLabel="下移代理组 Proxy"
        mobileMenuLabel="Proxy 更多操作"
        onDelete={onDelete}
        onDown={onDown}
        onUp={onUp}
        upDisabled
        upLabel="上移代理组 Proxy"
      />,
    );

    expect(screen.getByRole("button", { name: "上移代理组 Proxy" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "下移代理组 Proxy" })).toBeEnabled();
    await user.click(screen.getByRole("button", { name: "Proxy 更多操作" }));
    expect(screen.getByRole("menuitem", { name: "上移代理组 Proxy" })).toHaveAttribute("aria-disabled", "true");
    await user.click(screen.getByRole("menuitem", { name: "下移代理组 Proxy" }));
    expect(onDown).toHaveBeenCalledTimes(1);
    expect(onUp).not.toHaveBeenCalled();
    expect(onDelete).not.toHaveBeenCalled();
  });

  it("renders a driver-owned issue message descriptor without knowing its code", () => {
    localStorage.setItem("sandrone.locale", "en-US");
    render(
      <SectionIssues issues={[{
        code: "driver_reserved_name",
        itemId: "group-0",
        message: "untranslated fallback",
        messageKey: "files.config.issueShadowrocketNodeReserved",
        messageParams: { reference: "DIRECT" },
        section: "groups",
        severity: "error",
      }]} />,
    );

    expect(screen.getByRole("alert")).toHaveTextContent(
      "Rendered Shadowrocket node name “DIRECT” conflicts with a built-in policy.",
    );
    expect(screen.getByRole("alert")).not.toHaveTextContent("untranslated fallback");
  });
});

describe("ConfigWorkbenchSection", () => {
  it("is collapsed by default and wires the trigger to its stable content id", () => {
    render(
      <ConfigWorkbenchSection id="proxy-groups" label="Proxy groups">
        <p>Group editor</p>
      </ConfigWorkbenchSection>,
    );

    const trigger = screen.getByRole("button", { name: /Proxy groups/ });
    const content = document.getElementById("proxy-groups-content");

    expect(trigger).toHaveAttribute("id", "proxy-groups-header");
    expect(trigger).toHaveAttribute("aria-expanded", "false");
    expect(trigger).toHaveAttribute("aria-controls", "proxy-groups-content");
    expect(content).toHaveAttribute("aria-labelledby", "proxy-groups-header");
    expect(content).not.toBeVisible();
  });

  it("supports an initially expanded section with count, severity, and summary", () => {
    render(
      <ConfigWorkbenchSection
        defaultExpanded
        count={21}
        id="routing-rules"
        label="Routing rules"
        severity="warning"
        severityLabel="Needs attention"
        summary="2 invalid entries"
      >
        <p>Rule editor</p>
      </ConfigWorkbenchSection>,
    );

    const trigger = screen.getByRole("button", { name: /Routing rules/ });

    expect(trigger).toHaveAttribute("aria-expanded", "true");
    const info = document.querySelector('[data-slot="section-info"]');
    expect(info).not.toBeNull();
    expect(within(info as HTMLElement).getByText("21")).toBeInTheDocument();
    expect(within(info as HTMLElement).getByText("Needs attention")).toBeInTheDocument();
    expect(within(info as HTMLElement).getByText("2 invalid entries")).toBeInTheDocument();
    expect(screen.getByRole("region", { name: /Routing rules/ })).toBeVisible();
    expect(screen.getByText("Rule editor")).toBeVisible();
  });

  it("renders a numeric zero summary", () => {
    render(
      <ConfigWorkbenchSection defaultExpanded id="empty-rules" label="Rules" summary={0}>
        <p>No rules</p>
      </ConfigWorkbenchSection>,
    );

    expect(document.querySelector('[data-slot="section-info"]')).toHaveTextContent("0");
  });

  it("keeps controlled expansion under parent ownership", async () => {
    const user = userEvent.setup();
    const onExpandedChange = vi.fn();
    const { rerender } = render(
      <ConfigWorkbenchSection expanded={false} id="advanced" label="Advanced" onExpandedChange={onExpandedChange}>
        <p>Advanced editor</p>
      </ConfigWorkbenchSection>,
    );

    const trigger = screen.getByRole("button", { name: /Advanced/ });
    await user.click(trigger);

    expect(onExpandedChange).toHaveBeenCalledWith(true);
    expect(trigger).toHaveAttribute("aria-expanded", "false");

    rerender(
      <ConfigWorkbenchSection expanded id="advanced" label="Advanced" onExpandedChange={onExpandedChange}>
        <p>Advanced editor</p>
      </ConfigWorkbenchSection>,
    );

    expect(trigger).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByText("Advanced editor")).toBeVisible();
  });

  it("lets multiple uncontrolled sections remain expanded", async () => {
    const user = userEvent.setup();
    render(
      <>
        <ConfigWorkbenchSection id="groups" label="Groups"><p>Groups editor</p></ConfigWorkbenchSection>
        <ConfigWorkbenchSection id="rules" label="Rules"><p>Rules editor</p></ConfigWorkbenchSection>
      </>,
    );

    const groups = screen.getByRole("button", { name: /Groups/ });
    const rules = screen.getByRole("button", { name: /Rules/ });
    await user.click(groups);
    await user.click(rules);

    expect(groups).toHaveAttribute("aria-expanded", "true");
    expect(rules).toHaveAttribute("aria-expanded", "true");
  });

  it("toggles from the keyboard and reports each uncontrolled change", async () => {
    const user = userEvent.setup();
    const onExpandedChange = vi.fn();
    render(
      <ConfigWorkbenchSection id="sources" label="Sources" onExpandedChange={onExpandedChange}>
        <p>Source editor</p>
      </ConfigWorkbenchSection>,
    );

    const trigger = screen.getByRole("button", { name: /Sources/ });
    trigger.focus();
    await user.keyboard("{Enter}");
    expect(trigger).toHaveAttribute("aria-expanded", "true");
    expect(onExpandedChange).toHaveBeenLastCalledWith(true);

    await user.keyboard(" ");
    expect(trigger).toHaveAttribute("aria-expanded", "false");
    expect(onExpandedChange).toHaveBeenLastCalledWith(false);
  });

  it("keeps flexible status information from displacing actions or the rightmost collapse trigger", async () => {
    const user = userEvent.setup();
    const onAdd = vi.fn();
    render(
      <ConfigWorkbenchSection
        headerActions={<button type="button" onClick={onAdd}>Add</button>}
        id="groups"
        label="Groups"
        severity="warning"
        severityLabel="A localized status label that may become much longer"
      >
        <p>Groups editor</p>
      </ConfigWorkbenchSection>,
    );

    const trigger = screen.getByRole("button", { name: /Groups/ });
    const add = screen.getByRole("button", { name: "Add" });
    const info = document.querySelector('[data-slot="section-info"]');
    const actions = add.closest('[data-slot="section-actions"]');

    expect(trigger).not.toContainElement(add);
    expect(info).toHaveClass("min-w-0", "flex-1");
    expect(info).toContainElement(screen.getByText("A localized status label that may become much longer"));
    expect(actions).toHaveClass("shrink-0");
    expect(actions).not.toHaveClass("w-full");
    expect(actions?.compareDocumentPosition(trigger) ?? 0).toBe(Node.DOCUMENT_POSITION_FOLLOWING);

    await user.click(add);
    expect(onAdd).toHaveBeenCalledOnce();
    expect(trigger).toHaveAttribute("aria-expanded", "false");
  });
});

const referenceOptions: ConfigReferenceOption[] = [
  { kind: "macro", value: "$nodes" },
  { kind: "node", value: "HK Node", detail: "ss · hk.example:8388" },
  { kind: "group", value: "Auto" },
  { kind: "builtin", value: "DIRECT" },
];

describe("config reference fields", () => {
  it("preserves duplicates while adding, moving, and deleting ordered references", async () => {
    const user = userEvent.setup();
    render(<OrderedReferencesHarness />);

    expect(referenceValues()).toEqual(["$nodes", "DIRECT", "DIRECT"]);
    expect(screen.getByRole("button", { name: "添加成员" }).parentElement)
      .toHaveClass("flex", "justify-end");
    expect(screen.getByRole("button", { name: "拖动成员 1" })).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "下移成员 1" }));
    expect(referenceValues()).toEqual(["DIRECT", "$nodes", "DIRECT"]);
    await user.click(screen.getByRole("button", { name: "删除成员 3" }));
    expect(referenceValues()).toEqual(["DIRECT", "$nodes"]);
    await user.click(screen.getByRole("button", { name: "添加成员" }));

    const added = screen.getByRole("combobox", { name: "成员 3" });
    await user.click(added);
    fireEvent.change(added, { target: { value: "Custom Node" } });
    await user.keyboard("{Enter}");
    expect(referenceValues()).toEqual(["DIRECT", "$nodes", "Custom Node"]);
  });
});

function OrderedReferencesHarness() {
  const [values, setValues] = useState(["$nodes", "DIRECT", "DIRECT"]);
  return (
    <>
      <OrderedReferenceList label="成员" options={referenceOptions} values={values} onChange={setValues} />
      <output aria-label="引用值">{JSON.stringify(values)}</output>
    </>
  );
}

function referenceValues(): string[] {
  return JSON.parse(screen.getByRole("status", { name: "引用值" }).textContent ?? "[]") as string[];
}

const choices: ConfigTemplateChoice[] = [
  {
    description: "A small starting point",
    groupCount: 2,
    id: "minimal",
    name: "Minimal",
    ruleCount: 12,
    ruleSetCount: 1,
  },
  {
    description: "Balanced defaults for daily use",
    groupCount: 4,
    id: "standard",
    name: "Standard",
    ruleCount: 48,
    ruleSetCount: 3,
  },
  {
    description: "The complete routing set",
    groupCount: 8,
    id: "complete",
    name: "Complete",
    ruleCount: 96,
    ruleSetCount: 7,
  },
];

describe("ConfigTemplatePicker", () => {
  it("renders three accessible card-style radios with the current and customized states", () => {
    render(
      <ConfigTemplatePicker
        choices={choices}
        currentTemplateId="standard"
        customized
        onRequestApply={() => undefined}
      />,
    );

    const group = screen.getByRole("radiogroup", { name: "Configuration template" });
    const radios = within(group).getAllByRole("radio");

    expect(radios).toHaveLength(3);
    expect(screen.getByRole("radio", { name: "Standard" })).toBeChecked();
    expect(screen.getByRole("radio", { name: "Standard" })).toHaveAccessibleDescription(
      "Balanced defaults for daily use 4 groups · 3 rule sets · 48 rules",
    );
    expect(screen.getByRole("radio", { name: "Minimal" })).not.toBeChecked();
    expect(screen.getByText("Customized")).toBeInTheDocument();
    expect(group).toHaveAccessibleDescription("Customized");
    expect(screen.getByText("Balanced defaults for daily use")).toBeInTheDocument();
    expect(screen.getByText("4 groups")).toBeInTheDocument();
    expect(screen.getByText("3 rule sets")).toBeInTheDocument();
    expect(screen.getByText("48 rules")).toBeInTheDocument();
    expect(screen.getByText("1 rule set")).toBeInTheDocument();
  });

  it("requests a direct apply when confirmation is disabled", async () => {
    const user = userEvent.setup();
    const onRequestApply = vi.fn();
    render(
      <ConfigTemplatePicker
        choices={choices}
        currentTemplateId="standard"
        onRequestApply={onRequestApply}
      />,
    );

    await user.click(screen.getByRole("radio", { name: "Minimal" }));

    expect(onRequestApply).toHaveBeenCalledWith(choices[0]);
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("shows replacement counts and applies only after confirmation", async () => {
    const user = userEvent.setup();
    const onRequestApply = vi.fn();
    render(
      <ConfigTemplatePicker
        choices={choices}
        confirmBeforeApply
        currentTemplateId="standard"
        customized
        onRequestApply={onRequestApply}
      />,
    );

    await user.click(screen.getByRole("radio", { name: "Complete" }));

    const dialog = screen.getByRole("dialog", { name: "Replace configuration?" });
    expect(onRequestApply).not.toHaveBeenCalled();
    expect(within(dialog).getByText("Complete")).toBeInTheDocument();
    expect(within(dialog).getByText("8 groups")).toBeInTheDocument();
    expect(within(dialog).getByText("7 rule sets")).toBeInTheDocument();
    expect(within(dialog).getByText("96 rules")).toBeInTheDocument();

    await user.click(within(dialog).getByRole("button", { name: "Cancel" }));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(onRequestApply).not.toHaveBeenCalled();

    await user.click(screen.getByRole("radio", { name: "Complete" }));
    const replace = screen.getByRole("button", { name: "Replace configuration" });
    expect(replace).toHaveTextContent("Replace");
    await user.click(replace);

    expect(onRequestApply).toHaveBeenCalledWith(choices[2]);
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("uses native radio keyboard navigation to select a template", async () => {
    const user = userEvent.setup();
    const onRequestApply = vi.fn();

    function Harness() {
      const [currentTemplateId, setCurrentTemplateId] = useState("minimal");
      return (
        <ConfigTemplatePicker
          choices={choices}
          currentTemplateId={currentTemplateId}
          onRequestApply={(choice) => {
            onRequestApply(choice);
            setCurrentTemplateId(choice.id);
          }}
        />
      );
    }

    render(<Harness />);
    const minimal = screen.getByRole("radio", { name: "Minimal" });
    minimal.focus();
    await user.keyboard("{ArrowRight}");

    expect(screen.getByRole("radio", { name: "Standard" })).toBeChecked();
    expect(onRequestApply).toHaveBeenCalledWith(choices[1]);
  });

  it("accepts localized count formatters with language-specific word order", () => {
    render(
      <ConfigTemplatePicker
        choices={[choices[0]]}
        copy={{
          groups: (count) => `代理组 ${count}`,
          ruleSets: (count) => `规则集 ${count}`,
          rules: (count) => `规则 ${count}`,
        }}
        currentTemplateId="minimal"
        onRequestApply={() => undefined}
      />,
    );

    expect(screen.getByRole("radio", { name: "Minimal" })).toHaveAccessibleDescription(
      "A small starting point 代理组 2 · 规则集 1 · 规则 12",
    );
  });
});

describe("ConfigTemplateAppliedNotice", () => {
  it("renders a compact optional undo action without owning business state", async () => {
    const user = userEvent.setup();
    const onUndo = vi.fn();
    render(
      <ConfigTemplateAppliedNotice message="Standard template applied." undoLabel="Undo template change" onUndo={onUndo} />,
    );

    const notice = screen.getByRole("status");
    expect(notice).toHaveTextContent("Standard template applied.");
    await user.click(within(notice).getByRole("button", { name: "Undo template change" }));
    expect(onUndo).toHaveBeenCalledTimes(1);
  });
});

it("loads a target once, searches locally, and renders at most 100 matches", async () => {
  localStorage.setItem("sandrone.locale", "en-US");
  const user = userEvent.setup();
  const items = Array.from({ length: 101 }, (_, index) => catalogItem(`geosite-${String(index).padStart(3, "0")}`));
  const special = catalogItem("geosite-special");
  items.push(special);
  const loadCatalog = vi.fn().mockResolvedValue({ items });
  const onAdd = vi.fn().mockReturnValue({ status: "added", ruleSets: [] });
  const onClose = vi.fn();

  render(<RuleSetCatalogDialog kind="mihomo" open loadCatalog={loadCatalog} onAdd={onAdd} onClose={onClose} />);

  expect(screen.getByRole("dialog", { name: "Rule set catalog" })).toBeInTheDocument();
  expect(await screen.findByText("geosite-000")).toBeInTheDocument();
  expect(loadCatalog).toHaveBeenCalledOnce();
  expect(loadCatalog).toHaveBeenCalledWith("mihomo");
  expect(screen.getAllByRole("listitem")).toHaveLength(100);
  expect(screen.getByText(/Showing the first 100 matches/)).toBeInTheDocument();

  const search = screen.getByRole("textbox", { name: "Search rule sets" });
  expect(screen.getByText("Search", { selector: "label" })).toBeInTheDocument();
  fireEvent.change(search, { target: { value: "special" } });
  const row = await screen.findByRole("listitem");
  expect(within(row).getByText(special.name)).toBeInTheDocument();
  expect(within(row).getByText(special.url)).toBeInTheDocument();
  expect(loadCatalog).toHaveBeenCalledOnce();

  const add = within(row).getByRole("button", { name: `Add rule set “${special.name}”` });
  expect(add).toHaveTextContent("Add");
  await user.click(add);
  expect(onAdd).toHaveBeenCalledWith({ entry: special });
  expect(onClose).toHaveBeenCalledOnce();
});

it("reuses the loaded target after closing and reopening", async () => {
  localStorage.setItem("sandrone.locale", "en-US");
  const user = userEvent.setup();
  const loadCatalog = vi.fn().mockResolvedValue({ items: [catalogItem("geosite-cn")] });
  const props = {
    kind: "mihomo" as const,
    loadCatalog,
    onAdd: vi.fn().mockReturnValue({ status: "name-conflict", existingName: "existing" }),
    onClose: vi.fn(),
  };
  const { rerender } = render(<RuleSetCatalogDialog {...props} open />);

  expect(await screen.findByText("geosite-cn")).toBeInTheDocument();
  fireEvent.change(screen.getByRole("textbox", { name: "Search rule sets" }), { target: { value: "cn" } });
  await user.click(screen.getByRole("button", { name: "Add rule set “geosite-cn”" }));
  expect(screen.getByText(/already used by another URL/)).toBeInTheDocument();
  rerender(<RuleSetCatalogDialog {...props} open={false} />);
  rerender(<RuleSetCatalogDialog {...props} open />);
  expect(await screen.findByText("geosite-cn")).toBeInTheDocument();
  expect(loadCatalog).toHaveBeenCalledOnce();
  expect(screen.getByRole("textbox", { name: "Search rule sets" })).toHaveValue("");
  expect(screen.queryByText(/already used by another URL/)).not.toBeInTheDocument();
});

it("shows loading and request error states", async () => {
  localStorage.setItem("sandrone.locale", "en-US");
  const request = deferred<RuleSetCatalogResult>();
  render(<RuleSetCatalogDialog kind="sing-box" open loadCatalog={vi.fn().mockReturnValue(request.promise)} onAdd={vi.fn()} onClose={vi.fn()} />);

  expect(screen.getByLabelText("Loading rule-set catalog")).toBeInTheDocument();
  await act(async () => {
    request.reject(new Error("Catalog snapshot unavailable"));
    await request.promise.catch(() => undefined);
  });
  expect(await screen.findByText("Catalog snapshot unavailable")).toBeInTheDocument();
  expect(screen.queryByText("No matching rule sets.")).not.toBeInTheDocument();
});

it("shows local empty and conflict states without closing", async () => {
  localStorage.setItem("sandrone.locale", "en-US");
  const user = userEvent.setup();
  const duplicate = catalogItem("geosite-duplicate");
  const conflicting = catalogItem("geosite-conflict");
  const onAdd = vi.fn()
    .mockReturnValueOnce({ status: "duplicate-url", existingName: "existing-url" })
    .mockReturnValueOnce({ status: "name-conflict", existingName: "existing-name" });
  const onClose = vi.fn();
  render(<RuleSetCatalogDialog kind="mihomo" open loadCatalog={vi.fn().mockResolvedValue({ items: [duplicate, conflicting] })} onAdd={onAdd} onClose={onClose} />);

  const rows = await screen.findAllByRole("listitem");
  await user.click(within(rows[0]).getByRole("button", { name: "Add rule set “geosite-duplicate”" }));
  expect(screen.getByText("This URL already exists as “existing-url”.")).toBeInTheDocument();
  await user.click(within(rows[1]).getByRole("button", { name: "Add rule set “geosite-conflict”" }));
  expect(screen.getByText("The name “existing-name” is already used by another URL.")).toBeInTheDocument();
  expect(onClose).not.toHaveBeenCalled();

  fireEvent.change(screen.getByRole("textbox", { name: "Search rule sets" }), { target: { value: "missing" } });
  expect(await screen.findByText("No matching rule sets.")).toBeInTheDocument();
});

function catalogItem(name: string): RuleSetCatalogItem {
  return {
    name,
    ruleKind: "domain",
    url: `https://raw.githubusercontent.com/example/catalog/main/${name}.mrs`,
  };
}

function deferred<T>(): { promise: Promise<T>; reject: (reason: unknown) => void } {
  let reject!: (reason: unknown) => void;
  const promise = new Promise<T>((_resolve, fail) => { reject = fail; });
  return { promise, reject };
}
