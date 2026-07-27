import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { RenderCachePolicyField } from "./render-cache-policy-field";

describe("RenderCachePolicyField", () => {
  it("preserves explicit disable and exposes custom TTL input on demand", () => {
    render(<RenderCachePolicyField defaultValue={0} />);

    const policy = screen.getByRole("combobox", { name: "渲染结果缓存" });
    expect(policy).toHaveValue("disabled");
    expect(screen.queryByRole("spinbutton", { name: "渲染结果缓存时长（秒）" })).not.toBeInTheDocument();

    fireEvent.change(policy, { target: { value: "custom" } });
    expect(screen.getByRole("spinbutton", { name: "渲染结果缓存时长（秒）" })).toBeInTheDocument();
  });
});
