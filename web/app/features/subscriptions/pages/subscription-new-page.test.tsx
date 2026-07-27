import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { noop, subscriptions } from "~/features/subscriptions/test-data";

import { SubscriptionNewPage } from "./subscription-new-page";

describe("SubscriptionNewPage", () => {
  it("copies the current remote subscription URL from the form", async () => {
    const user = userEvent.setup();
    const onCopySource = vi.fn(async (_value: string, _target: "content" | "url") => undefined);

    render(<SubscriptionNewPage sources={subscriptions} type="remote" onBack={noop} onCopySource={onCopySource} onSave={noop} onTypeChange={noop} />);

    fireEvent.change(screen.getByRole("textbox", { name: "订阅地址" }), {
      target: { value: "https://example.com/sub" },
    });
    const copyURL = screen.getByRole("button", { name: "复制订阅地址" });
    await user.hover(copyURL);
    expect(await screen.findByRole("tooltip")).toHaveTextContent("复制");
    await user.click(copyURL);

    expect(onCopySource).toHaveBeenCalledWith("https://example.com/sub", "url");
  });
  it("copies the current local subscription content from the form", async () => {
    const user = userEvent.setup();
    const onCopySource = vi.fn(async (_value: string, _target: "content" | "url") => undefined);

    render(<SubscriptionNewPage sources={subscriptions} type="local" onBack={noop} onCopySource={onCopySource} onSave={noop} onTypeChange={noop} />);

    fireEvent.change(screen.getByRole("textbox", { name: "内容" }), {
      target: { value: "ss://example" },
    });
    const copyContent = screen.getByRole("button", { name: "复制内容" });
    await user.hover(copyContent);
    expect(await screen.findByRole("tooltip")).toHaveTextContent("复制");
    await user.click(copyContent);

    expect(onCopySource).toHaveBeenCalledWith("ss://example", "content");
  });
});
