import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { ConfigWorkbenchSection } from "./workbench-section";

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
