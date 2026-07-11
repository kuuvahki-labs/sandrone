import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { noop, subscriptions } from "~/features/subscriptions/test-data";

import { SubscriptionNewPage } from "./subscription-new-page";

describe("SubscriptionNewPage", () => {
  it("renders a remote subscription creation page", async () => {
    const user = userEvent.setup();
    const onSave = vi.fn(async (_form: FormData) => undefined);
    const onTypeChange = vi.fn();

    render(<SubscriptionNewPage sources={subscriptions} type="remote" onBack={noop} onSave={onSave} onTypeChange={onTypeChange} />);

    expect(screen.getByRole("heading", { name: "新建订阅" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "保存订阅" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "远程" })).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByRole("textbox", { name: "订阅地址" })).toHaveAttribute("name", "source_input");
    expect(screen.getByRole("combobox", { name: "格式" })).toHaveValue("auto");

    await user.type(screen.getByRole("textbox", { name: "订阅地址" }), "https://example.com/sub");
    await user.click(screen.getByRole("button", { name: "保存订阅" }));

    const saved = onSave.mock.calls[0]?.[0] as FormData;
    expect(saved.get("subscription_type")).toBe("remote");
    expect(saved.get("source_input")).toBe("https://example.com/sub");
  });
  it("copies the current remote subscription URL from the form", async () => {
    const user = userEvent.setup();
    const onCopySource = vi.fn(async (_value: string, _target: "content" | "url") => undefined);

    render(<SubscriptionNewPage sources={subscriptions} type="remote" onBack={noop} onCopySource={onCopySource} onSave={noop} onTypeChange={noop} />);

    await user.type(screen.getByRole("textbox", { name: "订阅地址" }), "https://example.com/sub");
    const copyURL = screen.getByRole("button", { name: "复制订阅地址" });
    await user.hover(copyURL);
    expect(await screen.findByRole("tooltip")).toHaveTextContent("复制");
    await user.click(copyURL);

    expect(onCopySource).toHaveBeenCalledWith("https://example.com/sub", "url");
  });
  it("renders a local subscription creation page", async () => {
    const user = userEvent.setup();
    const onSave = vi.fn(async (_form: FormData) => undefined);

    render(<SubscriptionNewPage sources={subscriptions} type="local" onBack={noop} onSave={onSave} onTypeChange={noop} />);

    expect(screen.getByRole("button", { name: "本地" })).toHaveAttribute("aria-pressed", "true");
    const contentInput = screen.getByRole("textbox", { name: "内容" });
    const contentEditor = contentInput.closest("[data-highlighted-textarea]");
    expect(contentEditor).toHaveAttribute("data-highlighted-textarea", "text");
    expect(contentEditor?.querySelector('[data-line-number="1"]')).toHaveTextContent("1");
    expect(screen.getByRole("textbox", { name: "名称" })).toHaveValue("manual");

    await user.type(contentInput, "ss://example");
    await user.click(screen.getByRole("button", { name: "保存订阅" }));

    const saved = onSave.mock.calls[0]?.[0] as FormData;
    expect(saved.get("subscription_type")).toBe("local");
    expect(saved.get("source_input")).toBe("ss://example");
  });
  it("copies the current local subscription content from the form", async () => {
    const user = userEvent.setup();
    const onCopySource = vi.fn(async (_value: string, _target: "content" | "url") => undefined);

    render(<SubscriptionNewPage sources={subscriptions} type="local" onBack={noop} onCopySource={onCopySource} onSave={noop} onTypeChange={noop} />);

    await user.type(screen.getByRole("textbox", { name: "内容" }), "ss://example");
    const copyContent = screen.getByRole("button", { name: "复制内容" });
    await user.hover(copyContent);
    expect(await screen.findByRole("tooltip")).toHaveTextContent("复制");
    await user.click(copyContent);

    expect(onCopySource).toHaveBeenCalledWith("ss://example", "content");
  });
  it("renders a collection subscription creation page", async () => {
    const user = userEvent.setup();
    const onSave = vi.fn(async (_form: FormData) => undefined);

    render(<SubscriptionNewPage sources={subscriptions} type="collection" onBack={noop} onSave={onSave} onTypeChange={noop} />);

    expect(screen.getByRole("button", { name: "组合" })).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByRole("textbox", { name: "名称" })).toHaveValue("default");
    const sourcePicker = screen.getByRole("group", { name: "包含订阅" });
    await user.click(within(sourcePicker).getByRole("checkbox", { name: "provider 远程订阅 · uri-list" }));
    await user.click(within(sourcePicker).getByRole("checkbox", { name: "warn 远程订阅 · uri-list" }));
    await user.click(screen.getByRole("button", { name: "保存订阅" }));

    const saved = onSave.mock.calls[0]?.[0] as FormData;
    expect(saved.get("subscription_type")).toBe("collection");
    expect(saved.getAll("subscriptions")).toEqual(["provider", "warn"]);
  });
});
