import { useState } from "react";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";

import { ProbeURLField } from "./probe-url-field";

describe("ProbeURLField", () => {
  it("offers the ordered probe providers and selects their complete URL", async () => {
    const user = userEvent.setup();
    render(<ProbeURLHarness initialValue="http://www.gstatic.com/generate_204" />);

    const input = screen.getByRole("combobox", { name: "URL" });
    await user.click(input);
    await user.keyboard("{ArrowDown}");

    const listbox = await screen.findByRole("listbox");
    expect(within(listbox).getAllByRole("option").map((option) => option.textContent)).toEqual([
      "Googlehttp://www.gstatic.com/generate_204",
      "Applehttp://captive.apple.com/hotspot-detect.html",
      "Cloudflarehttp://cp.cloudflare.com/generate_204",
      "Microsofthttp://www.msftconnecttest.com/connecttest.txt",
      "华为http://connectivitycheck.platform.hicloud.com/generate_204",
    ]);

    await user.click(within(listbox).getByRole("option", {
      name: "Apple http://captive.apple.com/hotspot-detect.html",
    }));
    expect(input).toHaveValue("http://captive.apple.com/hotspot-detect.html");
    expect(screen.getByRole("status", { name: "当前 URL" }))
      .toHaveTextContent("http://captive.apple.com/hotspot-detect.html");
  });

  it("keeps a URL outside the preset catalog", async () => {
    const user = userEvent.setup();
    render(<ProbeURLHarness initialValue="https://probe.example.test/ok" />);

    const input = screen.getByRole("combobox", { name: "URL" });
    expect(input).toHaveValue("https://probe.example.test/ok");
    await user.clear(input);
    await user.type(input, "https://custom.example.test/health");
    await user.tab();

    expect(input).toHaveValue("https://custom.example.test/health");
    expect(screen.getByRole("status", { name: "当前 URL" }))
      .toHaveTextContent("https://custom.example.test/health");
  });
});

function ProbeURLHarness({ initialValue }: { initialValue: string }) {
  const [value, setValue] = useState(initialValue);
  return (
    <>
      <ProbeURLField label="URL" value={value} onChange={setValue} />
      <output aria-label="当前 URL">{value}</output>
    </>
  );
}
