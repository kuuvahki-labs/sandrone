import { useState } from "react";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import {
  ConfigTemplateAppliedNotice,
  type ConfigTemplateChoice,
  ConfigTemplatePicker,
} from "./template-picker";

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
