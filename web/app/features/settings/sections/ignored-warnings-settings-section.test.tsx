import { useState } from "react";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";

import type { IgnoredWarning } from "~/shared/resources/types";

import { IgnoredWarningsSettingsSection } from "./ignored-warnings-settings-section";

describe("ignored warnings settings section", () => {
  it("groups field rules and restores one field or all rules from the settings draft", async () => {
    const user = userEvent.setup();
    render(<Harness />);

    expect(screen.getAllByText("uri.query.mode")).toHaveLength(1);
    expect(screen.getByText("2 条规则")).toBeInTheDocument();
    expect(screen.getByText("probe_timeout")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "恢复 uri.query.mode" }));
    expect(screen.queryByText("uri.query.mode")).not.toBeInTheDocument();
    expect(screen.queryByText("parse_unknown_field")).not.toBeInTheDocument();
    expect(screen.queryByText("render_lossy_field")).not.toBeInTheDocument();
    expect(screen.getByText("probe_timeout")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "全部恢复" }));
    expect(screen.getByText("没有已忽略的订阅警告。")).toBeInTheDocument();
  });
});

const initialWarnings: IgnoredWarning[] = [{
  code: "parse_unknown_field",
  field: "uri.query.mode",
  source: "uri-list",
}, {
  code: "render_lossy_field",
  field: "uri.query.mode",
  target: "sing-box-outbounds",
}, {
  code: "probe_timeout",
}];

function Harness() {
  const [warnings, setWarnings] = useState(initialWarnings);
  return <IgnoredWarningsSettingsSection value={warnings} onChange={setWarnings} />;
}
