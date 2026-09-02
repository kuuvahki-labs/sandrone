import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import type { SubscriptionItem } from "~/features/subscriptions/model/types";
import { noop, probeCacheTTLSeconds, probeDefaults, subscriptions } from "~/features/subscriptions/test-data";

import { SubscriptionNewPage } from "./subscription-new-page";

describe("SubscriptionNewPage", () => {
  it("requires confirmation when a generated remote name matches an existing subscription", async () => {
    const user = userEvent.setup();
    const existing: SubscriptionItem = {
      kind: "remote",
      name: "example.com",
      title: "example.com",
      label: "远程订阅",
      status: "ready",
      createdAt: "2026-06-27T01:02:03.000Z",
    };
    const onSave = vi.fn(async (_form: FormData, _existing?: SubscriptionItem) => undefined);

    render(
      <SubscriptionNewPage
        probeCacheTTLSeconds={probeCacheTTLSeconds}
        probeDefaults={probeDefaults}
        sources={[existing]}
        type="remote"
        onBack={noop}
        onSave={onSave}
        onTypeChange={noop}
      />,
    );

    await user.type(screen.getByRole("textbox", { name: "订阅地址" }), "https://www.example.com/sub");
    await user.click(screen.getByRole("button", { name: "保存订阅" }));

    expect(onSave).not.toHaveBeenCalled();
    expect(screen.getByRole("dialog", { name: "覆盖同名订阅？" })).toHaveTextContent(
      "订阅“example.com”已经存在。继续操作会完整覆盖现有订阅。",
    );

    await user.click(screen.getByRole("button", { name: "覆盖" }));

    expect(onSave).toHaveBeenCalledTimes(1);
    expect(onSave.mock.calls[0]?.[1]).toBe(existing);
  });

  it("copies the current remote subscription URL from the form", async () => {
    const user = userEvent.setup();
    const onCopySource = vi.fn(async (_value: string, _target: "content" | "url") => undefined);

    render(
      <SubscriptionNewPage
        probeCacheTTLSeconds={probeCacheTTLSeconds}
        probeDefaults={probeDefaults}
        remoteDefaults={{ cacheTTLSeconds: 300, proxy: "http://proxy.test", timeoutMS: 15000, userAgent: "Sandrone Global" }}
        sources={subscriptions}
        type="remote"
        onBack={noop}
        onCopySource={onCopySource}
        onSave={noop}
        onTypeChange={noop}
      />,
    );

    expect(screen.getByRole("textbox", { name: "User-Agent" })).toHaveAttribute("placeholder", "Sandrone Global");
    expect(screen.getByRole("textbox", { name: "代理" })).toHaveAttribute("placeholder", "http://proxy.test");
    expect(screen.getByRole("spinbutton", { name: "超时（秒）" })).toHaveAttribute("placeholder", "15");
    expect(screen.getByRole("spinbutton", { name: "远程请求缓存（秒）" })).toHaveAttribute("placeholder", "300");

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

    render(<SubscriptionNewPage probeCacheTTLSeconds={probeCacheTTLSeconds} probeDefaults={probeDefaults} sources={subscriptions} type="local" onBack={noop} onCopySource={onCopySource} onSave={noop} onTypeChange={noop} />);

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
