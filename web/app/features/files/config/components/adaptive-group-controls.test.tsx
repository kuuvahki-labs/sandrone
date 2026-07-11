import { useState } from "react";
import { fireEvent, render, screen } from "@testing-library/react";
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

    expect(screen.getByText("根据当前订阅预览按需创建运行时筛选分组。")).toBeInTheDocument();
  });

  it("submits only on click with persisted url-test, threshold-two, and five-region defaults", async () => {
    const user = userEvent.setup();
    const onGenerate = vi.fn();
    const view = render(
      <ControlsHarness generatedCount={0} onGenerate={onGenerate} />,
    );

    expect(screen.getByRole("combobox", { name: "代理组类型" })).toHaveTextContent("url-test");
    expect(screen.getByRole("spinbutton", { name: "地区最少节点数" })).toHaveValue(2);
    expect(onGenerate).not.toHaveBeenCalled();
    const generate = screen.getByRole("button", { name: "生成自适应分组" });
    expect(generate).toHaveTextContent("生成");
    await user.click(generate);
    expect(onGenerate).toHaveBeenCalledWith(adaptive.defaultOptions());

    view.unmount();
    render(<ControlsHarness generatedCount={1} onGenerate={onGenerate} />);
    expect(screen.getByRole("button", { name: "重新生成自适应分组" })).toHaveTextContent("重新生成");
    expect(screen.getByRole("spinbutton", { name: "地区最少节点数" })).toHaveValue(2);
  });

  it("persists edited options and submits them only after the generate click", async () => {
    const user = userEvent.setup();
    const onGenerate = vi.fn();
    const onOptionsChange = vi.fn();
    render(<ControlsHarness generatedCount={0} onGenerate={onGenerate} onOptionsChange={onOptionsChange} />);

    await choose(user, "代理组类型", "load-balance");
    fireEvent.change(screen.getByRole("spinbutton", { name: "地区最少节点数" }), { target: { value: "3" } });
    expect(onGenerate).not.toHaveBeenCalled();
    expect(onOptionsChange).toHaveBeenLastCalledWith(expect.objectContaining({
      type: "load-balance",
      minimumNodeCount: 3,
    }));
    await user.click(screen.getByRole("button", { name: "生成自适应分组" }));

    expect(onGenerate).toHaveBeenCalledWith(expect.objectContaining({
      type: "load-balance",
      minimumNodeCount: 3,
    }));
  });

  it.each(["0", "1.5", ""])("rejects the invalid minimum threshold %j", (value) => {
    render(<ControlsHarness generatedCount={0} onGenerate={vi.fn()} />);

    fireEvent.change(screen.getByRole("spinbutton", { name: "地区最少节点数" }), { target: { value } });

    expect(screen.getByRole("button", { name: "生成自适应分组" })).toBeDisabled();
    expect(screen.getByText("地区最少节点数必须是大于等于 1 的整数。")).toBeInTheDocument();
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
        ]}
        onGenerate={vi.fn()}
      />,
    );

    expect(screen.getByRole("alert")).toBeInTheDocument();
    expect(screen.getByText(/同名自定义组.*香港节点/)).toBeInTheDocument();
    expect(screen.getByText(/节点名.*日本节点.*冲突/)).toBeInTheDocument();
    expect(screen.getByText(/美国节点.*仍被其他配置引用/)).toBeInTheDocument();
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
    fireEvent.change(screen.getByRole("spinbutton", { name: "地区最少节点数" }), { target: { value: "4" } });

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
