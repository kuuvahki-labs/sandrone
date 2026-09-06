import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import type { LogSnapshot } from "~/shared/api/client";

import { SettingsLogsPage } from "./settings-logs-page";

function snapshot(count = 1): LogSnapshot {
  return {
    instance_id: "runtime-a", snapshot_time: "2026-09-06T01:02:03.456Z", level: "info",
    max_entries: 1000, max_bytes: 2097152, max_entry_bytes: 8192, dropped: 0,
    entries: Array.from({ length: count }, (_, index) => ({
      id: count - index, time: "2026-09-06T01:02:03.456Z", level: index % 2 === 0 ? "info" : "error",
      message: `event-${count - index}`, attrs: { operation: "fetch", token: "example-token" }, truncated: false,
    })),
  };
}

describe("program logs", () => {
  it("loads once, preserves a failed snapshot, and resets pagination only on success", async () => {
    const user = userEvent.setup();
    const getLogs = vi.fn().mockResolvedValueOnce(snapshot(101)).mockRejectedValueOnce(new Error("offline")).mockResolvedValueOnce(snapshot(102));
    render(<SettingsLogsPage client={{ getLogs }} onBack={vi.fn()} />);
    expect(await screen.findByLabelText("日志详情 101")).toBeInTheDocument();
    await user.type(screen.getByRole("textbox", { name: "搜索日志…" }), "fetch");
    await user.click(screen.getByRole("button", { name: /page 2/i }));
    expect(screen.getByLabelText("日志详情 1")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "刷新" }));
    expect(await screen.findByText("日志加载失败：offline")).toBeInTheDocument();
    expect(screen.getByLabelText("日志详情 1")).toBeInTheDocument();
    await user.click(screen.getAllByRole("button", { name: "刷新" })[0]!);
    expect(await screen.findByLabelText("日志详情 102")).toBeInTheDocument();
    expect(screen.getByRole("textbox")).toHaveValue("fetch");
    expect(screen.queryByText("日志加载失败：offline")).not.toBeInTheDocument();
    expect(getLogs).toHaveBeenCalledTimes(3);
  });

  it("filters levels and nested fields and displays raw details as text", async () => {
    const user = userEvent.setup();
    const data = snapshot(2);
    data.entries[0]!.message = '<script>alert("example")</script>';
    data.entries[0]!.truncated = true;
    data.entries[0]!.level = "debug";
    data.dropped = 12;
    render(<SettingsLogsPage client={{ getLogs: vi.fn().mockResolvedValue(data) }} onBack={vi.fn()} />);
    expect(await screen.findByText("较早日志已被覆盖（12 条）")).toBeInTheDocument();
    await user.click(screen.getByRole("combobox", { name: "筛选日志级别" }));
    await user.click(screen.getByRole("option", { name: "仅调试" }));
    expect(screen.queryByText("event-1")).not.toBeInTheDocument();
    await user.type(screen.getByRole("textbox"), "example-token");
    expect(screen.getByText("1 / 2 条")).toBeInTheDocument();
    const details = screen.getByLabelText("日志详情 2").closest("details")!;
    fireEvent.click(screen.getByLabelText("日志详情 2"));
    expect(details).toHaveAttribute("open");
    expect(details.querySelector("script")).toBeNull();
    expect(screen.getByText("此条日志过长，已截断")).toBeInTheDocument();
    await user.clear(screen.getByRole("textbox"));
    await user.type(screen.getByRole("textbox"), "absent");
    expect(screen.getByText("没有匹配的日志")).toBeInTheDocument();
  });

  it("shows an empty snapshot and aborts the pending refresh on unmount", async () => {
    let resolve!: (value: LogSnapshot) => void;
    const getLogs = vi.fn().mockResolvedValueOnce(snapshot(0)).mockImplementationOnce(() => new Promise<LogSnapshot>((done) => { resolve = done; }));
    const view = render(<SettingsLogsPage client={{ getLogs }} onBack={vi.fn()} />);
    expect(await screen.findByText("暂无日志")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "刷新" }));
    await waitFor(() => expect(getLogs).toHaveBeenCalledTimes(2));
    const signal = getLogs.mock.calls[1]![0] as AbortSignal;
    view.unmount();
    expect(signal.aborted).toBe(true);
    await act(async () => resolve(snapshot(1)));
  });

  it("does not poll while mounted", async () => {
    vi.useFakeTimers();
    try {
      const getLogs = vi.fn().mockResolvedValue(snapshot());
      const view = render(<SettingsLogsPage client={{ getLogs }} onBack={vi.fn()} />);
      await act(async () => { await vi.advanceTimersByTimeAsync(60_000); });
      expect(getLogs).toHaveBeenCalledTimes(1);
      view.unmount();
    } finally { vi.useRealTimers(); }
  });
});
