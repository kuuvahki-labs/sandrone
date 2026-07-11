import { useState } from "react";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";

import type { ConfigReferenceOption } from "~/features/files/config/model/references";

import { OrderedReferenceList } from "./reference-fields";

const options: ConfigReferenceOption[] = [
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
    expect(screen.getByRole("button", { name: "拖动成员 1" })).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "下移成员 1" }));
    expect(referenceValues()).toEqual(["DIRECT", "$nodes", "DIRECT"]);
    await user.click(screen.getByRole("button", { name: "删除成员 3" }));
    expect(referenceValues()).toEqual(["DIRECT", "$nodes"]);
    await user.click(screen.getByRole("button", { name: "添加成员" }));

    const added = screen.getByRole("combobox", { name: "成员 3" });
    await user.type(added, "Custom Node");
    await user.keyboard("{Enter}");
    expect(referenceValues()).toEqual(["DIRECT", "$nodes", "Custom Node"]);
  });
});

function OrderedReferencesHarness() {
  const [values, setValues] = useState(["$nodes", "DIRECT", "DIRECT"]);
  return (
    <>
      <OrderedReferenceList label="成员" options={options} values={values} onChange={setValues} />
      <output aria-label="引用值">{JSON.stringify(values)}</output>
    </>
  );
}

function referenceValues(): string[] {
  return JSON.parse(screen.getByRole("status", { name: "引用值" }).textContent ?? "[]") as string[];
}
