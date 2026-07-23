import { useState } from "react";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import type {
  AdaptiveGroupOptions,
  AdaptiveGroupWarning,
} from "~/features/files/config/model/adaptive-groups";
import { requireFileDriver } from "~/features/files/drivers/registry";

import { ConfigAdaptiveGroupControls } from "./adaptive-group-controls";

const driver = requireFileDriver("mihomo");
if (driver.configuration.mode !== "structured") {
  throw new Error("expected mihomo to use structured configuration");
}
const adaptive = driver.configuration.adapter.adaptive;

afterEach(() => {
  localStorage.removeItem("sandrone.locale");
});

describe("ConfigAdaptiveGroupControls", () => {
  it("keeps the adaptive-group description focused on what generation does", () => {
    render(<ControlsHarness generatedCount={0} onGenerate={vi.fn()} />);

    expect(screen.getByText("按所选地区创建分组；支持动态筛选的客户端无需节点预览。")).toBeInTheDocument();
  });

  it("submits only on click with persisted url-test and five-region defaults", async () => {
    const user = userEvent.setup();
    const onGenerate = vi.fn();
    const view = render(
      <ControlsHarness generatedCount={0} onGenerate={onGenerate} />,
    );

    expect(screen.getByRole("combobox", { name: "代理组类型" })).toHaveTextContent("url-test");
    expect(onGenerate).not.toHaveBeenCalled();
    const generate = screen.getByRole("button", { name: "生成自适应分组" });
    expect(generate).toHaveTextContent("生成");
    await user.click(generate);
    expect(onGenerate).toHaveBeenCalledWith(adaptive.defaultOptions());

    view.unmount();
    render(<ControlsHarness generatedCount={1} onGenerate={onGenerate} />);
    expect(screen.getByRole("button", { name: "重新生成自适应分组" })).toHaveTextContent("重新生成");
  });

  it("persists the edited group type and submits it only after the generate click", async () => {
    const user = userEvent.setup();
    const onGenerate = vi.fn();
    const onOptionsChange = vi.fn();
    render(<ControlsHarness generatedCount={0} onGenerate={onGenerate} onOptionsChange={onOptionsChange} />);

    await choose(user, "代理组类型", "load-balance");
    expect(onGenerate).not.toHaveBeenCalled();
    expect(onOptionsChange).toHaveBeenLastCalledWith(expect.objectContaining({
      type: "load-balance",
    }));
    await user.click(screen.getByRole("button", { name: "生成自适应分组" }));

    expect(onGenerate).toHaveBeenCalledWith(expect.objectContaining({
      type: "load-balance",
    }));
  });

  it("disables generation and shows an external reason", () => {
    render(
      <ControlsHarness
        disabledReason="正在加载节点预览。"
        generatedCount={0}
        onGenerate={vi.fn()}
      />,
    );

    expect(screen.getByRole("button", { name: "生成自适应分组" })).toBeDisabled();
    expect(screen.getByText("正在加载节点预览。")).toBeInTheDocument();
  });

  it("renders custom-name, node-name, and referenced-stale warnings", () => {
    render(
      <ControlsHarness
        generatedCount={1}
        warnings={[
          { code: "group_name_conflict", groupName: "香港节点" },
          { code: "node_name_conflict", groupName: "日本节点" },
          { code: "referenced_stale_group", groupName: "美国节点" },
          { code: "empty_regions_skipped", groupNames: ["台湾", "新加坡"] },
        ]}
        onGenerate={vi.fn()}
      />,
    );

    expect(screen.getByRole("alert")).toBeInTheDocument();
    expect(screen.getByText(/同名自定义组.*香港节点/)).toBeInTheDocument();
    expect(screen.getByText(/节点名.*日本节点.*冲突/)).toBeInTheDocument();
    expect(screen.getByText(/美国节点.*仍被其他配置引用/)).toBeInTheDocument();
    expect(screen.getByText(/没有匹配节点.*台湾, 新加坡/)).toBeInTheDocument();
  });

  it("stops transient input changes from reaching the parent dirty handler", async () => {
    const user = userEvent.setup();
    const parentChange = vi.fn();
    const onGenerate = vi.fn();
    render(
      <form onChange={parentChange}>
        <ControlsHarness generatedCount={0} onGenerate={onGenerate} />
      </form>,
    );

    await choose(user, "代理组类型", "select");

    expect(parentChange).not.toHaveBeenCalled();
    expect(onGenerate).not.toHaveBeenCalled();
  });

  it("selects five regions by default and supports clear, individual selection, and select all", async () => {
    const user = userEvent.setup();
    const onOptionsChange = vi.fn();
    render(<ControlsHarness generatedCount={0} onGenerate={vi.fn()} onOptionsChange={onOptionsChange} />);

    const scope = screen.getByRole("button", { name: /生成范围/ });
    expect(scope).toHaveAttribute("aria-expanded", "false");
    expect(screen.getByText("已选择 5 个地区")).toBeInTheDocument();
    expect(screen.queryByRole("checkbox", { name: /Hong Kong/ })).not.toBeInTheDocument();

    await user.click(scope);
    expect(scope).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByRole("checkbox", { name: /Hong Kong/ })).toBeChecked();
    expect(screen.getByRole("checkbox", { name: /Taiwan/ })).toBeChecked();
    expect(screen.getByRole("checkbox", { name: /Japan/ })).toBeChecked();
    expect(screen.getByRole("checkbox", { name: /United States/ })).toBeChecked();
    expect(screen.getByRole("checkbox", { name: /Singapore/ })).toBeChecked();
    expect(screen.getByRole("checkbox", { name: /South Korea/ })).not.toBeChecked();

    await user.click(screen.getByRole("button", { name: "清空" }));
    expect(screen.getByText("已选择 0 个地区")).toBeInTheDocument();
    expect(screen.getByRole("checkbox", { name: /Hong Kong/ })).not.toBeChecked();

    await user.click(screen.getByRole("checkbox", { name: /Hong Kong/ }));
    expect(screen.getByText("已选择 1 个地区")).toBeInTheDocument();

    expect(onOptionsChange).toHaveBeenLastCalledWith(expect.objectContaining({
      enabledRegionIds: ["hk"],
    }));

    await user.click(screen.getByRole("button", { name: "全选" }));
    expect(screen.getByText("已选择 22 个地区")).toBeInTheDocument();
  });

  it("shows selected regions first and sorts each selection bucket by matched node count", async () => {
    const user = userEvent.setup();
    render(<ControlsHarness generatedCount={0} onGenerate={vi.fn()} />);

    await user.click(screen.getByRole("button", { name: /生成范围/ }));
    const labels = screen.getAllByRole("checkbox").map((checkbox) => checkbox.closest("label")?.textContent ?? "");

    expect(labels.slice(0, 5)).toEqual([
      expect.stringMatching(/Hong Kong/),
      expect.stringMatching(/Japan/),
      expect.stringMatching(/Taiwan/),
      expect.stringMatching(/Singapore/),
      expect.stringMatching(/United States/),
    ]);
  });
});

function ControlsHarness({
  disabledReason,
  generatedCount,
  initialOptions = adaptive.defaultOptions(),
  onGenerate,
  onOptionsChange,
  warnings = [],
}: {
  disabledReason?: string;
  generatedCount: number;
  initialOptions?: AdaptiveGroupOptions;
  onGenerate: (options: AdaptiveGroupOptions) => void;
  onOptionsChange?: (options: AdaptiveGroupOptions) => void;
  warnings?: readonly AdaptiveGroupWarning[];
}) {
  const [options, setOptions] = useState(initialOptions);
  const candidates = adaptive.generate(["HK-01", "香港-02", "JP-01"], options).candidates;
  return (
    <ConfigAdaptiveGroupControls
      candidates={candidates}
      disabledReason={disabledReason}
      generatedCount={generatedCount}
      options={options}
      typeOptions={adaptive.typeOptions}
      warnings={warnings}
      onGenerate={onGenerate}
      onOptionsChange={(next) => {
        setOptions(next);
        onOptionsChange?.(next);
      }}
    />
  );
}

async function choose(
  user: ReturnType<typeof userEvent.setup>,
  label: string,
  option: string,
): Promise<void> {
  await user.click(screen.getByRole("combobox", { name: label }));
  await user.click(await screen.findByRole("option", { name: option }));
}
