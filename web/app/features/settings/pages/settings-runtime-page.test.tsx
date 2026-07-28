import { fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import type { RuntimeSettingsInput } from "~/shared/api/client";

import { SettingsRuntimePage } from "./settings-runtime-page";

const runtimeSettings: RuntimeSettingsInput = {
  remote_defaults: {
    user_agent: "sandrone/0.1.0",
    timeout_ms: 15000,
  },
  probe_defaults: {
    method: "url_test",
    core: "sing-box",
    url: "http://www.gstatic.com/generate_204",
    ntp_server: "time.apple.com",
    timeout_ms: 5000,
    attempts: 1,
    concurrency: 10,
    cache_ttl_seconds: 0,
  },
  cache_defaults: {
    remote_fetch_ttl_seconds: 0,
    subscription_traffic_ttl_seconds: 60,
    subscription_render_ttl_seconds: 0,
    file_render_ttl_seconds: 0,
  },
};

describe("settings runtime page", () => {
  it("shows the focused heading, returns, and initially expands only remote requests", async () => {
    const user = userEvent.setup();
    const onBack = vi.fn();

    render(
      <SettingsRuntimePage
        runtimeSettings={runtimeSettings}
        onBack={onBack}
        onSaveRuntimeSettings={vi.fn()}
      />,
    );

    expect(screen.getByRole("heading", { name: "运行默认值" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "返回" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "远程请求" })).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByRole("button", { name: "缓存" })).toHaveAttribute("aria-expanded", "false");
    expect(screen.getByRole("button", { name: "测活" })).toHaveAttribute("aria-expanded", "false");

    await user.click(screen.getByRole("button", { name: "返回" }));

    expect(onBack).toHaveBeenCalledTimes(1);
  });

  it("edits and saves runtime defaults", async () => {
    const user = userEvent.setup();
    const onSaveRuntimeSettings = vi.fn();
    render(
      <SettingsRuntimePage
        runtimeSettings={runtimeSettings}
        onBack={vi.fn()}
        onSaveRuntimeSettings={onSaveRuntimeSettings}
      />,
    );

    const remoteGroup = screen.getByRole("region", { name: "远程请求" });
    fireEvent.change(within(remoteGroup).getByRole("textbox", { name: "User-Agent" }), {
      target: { value: "Sandrone Global" },
    });

    await user.click(screen.getByRole("button", { name: "缓存" }));
    const cacheGroup = screen.getByRole("region", { name: "缓存" });
    fireEvent.change(within(cacheGroup).getByRole("spinbutton", { name: "远程请求缓存（秒）" }), {
      target: { value: "120" },
    });
    fireEvent.change(within(cacheGroup).getByRole("spinbutton", { name: "订阅流量缓存（秒）" }), {
      target: { value: "15" },
    });
    fireEvent.change(within(cacheGroup).getByRole("spinbutton", { name: "订阅渲染缓存（秒）" }), {
      target: { value: "180" },
    });
    fireEvent.change(within(cacheGroup).getByRole("spinbutton", { name: "文件渲染缓存（秒）" }), {
      target: { value: "240" },
    });

    await user.click(screen.getByRole("button", { name: "测活" }));
    const probeGroup = screen.getByRole("region", { name: "测活" });
    expect(within(probeGroup).getAllByRole("combobox")).toHaveLength(2);
    expect(probeGroup).not.toHaveTextContent(/sing-box|mihomo/);
    expect(within(probeGroup).getByRole("combobox", { name: "默认测活方式" })).toHaveTextContent("url_test");
    const probeURL = within(probeGroup).getByRole("combobox", { name: "URL" });
    await user.click(probeURL);
    await user.keyboard("{ArrowDown}");
    await user.click(await screen.findByRole("option", {
      name: "Cloudflare http://cp.cloudflare.com/generate_204",
    }));
    fireEvent.change(within(probeGroup).getByRole("spinbutton", { name: "缓存（秒）" }), {
      target: { value: "300" },
    });

    const saveRuntimeDefaults = screen.getByRole("button", { name: "保存运行默认值" });
    expect(saveRuntimeDefaults).toHaveTextContent("保存");
    await user.click(saveRuntimeDefaults);

    expect(onSaveRuntimeSettings).toHaveBeenCalledWith(expect.objectContaining({
      remote_defaults: expect.objectContaining({
        user_agent: "Sandrone Global",
        timeout_ms: 15000,
      }),
      probe_defaults: expect.objectContaining({
        cache_ttl_seconds: 300,
        core: "sing-box",
        url: "http://cp.cloudflare.com/generate_204",
      }),
      cache_defaults: expect.objectContaining({
        remote_fetch_ttl_seconds: 120,
        subscription_traffic_ttl_seconds: 15,
        subscription_render_ttl_seconds: 180,
        file_render_ttl_seconds: 240,
      }),
    }));
  });
});
