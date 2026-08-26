import { fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import type { FileItem } from "~/features/files/model/types";
import { createAction, files, noop } from "~/features/files/test-data";

import { FilePreviewPage } from "./file-preview-page";
import { FilesPage } from "./files-page";

const pageActions = {
  backLabel: "返回编辑",
  fileName: "default.txt",
  onBack: vi.fn(),
  onRefresh: vi.fn(),
  onShare: vi.fn(),
};

describe("FilePreviewPage", () => {
  it("shows final bodies directly without adding tabs or alerts", () => {
    const cases = [
    ["JSON", "application/json", "{\"future\":true}", "future"],
    ["YAML", "application/yaml", "rules:\n  - allow\n", "allow"],
    ["plain text", "text/plain", "plain final output", "plain final output"],
    ["empty", "application/octet-stream", "", ""],
    ] as const;
    const view = render(
      <FilePreviewPage
        {...pageActions}
        preview={{ body: cases[0][2], contentType: cases[0][1], warnings: [] }}
      />,
    );

    for (const [, contentType, body, expected] of cases) {
      view.rerender(
        <FilePreviewPage
          {...pageActions}
          preview={{ body, contentType, warnings: [] }}
        />,
      );
      const code = screen.getByRole("region", { name: "最终文件内容" }).querySelector("code");
      if (body) {
        expect(code?.textContent).toContain(expected);
      } else {
        expect(code?.textContent.trim()).toBe("");
      }
    }
    expect(screen.queryByRole("tab")).not.toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("uses JavaScript highlighting from either the filename or content type", () => {
    const view = render(
      <FilePreviewPage
        {...pageActions}
        fileName="rename.JS"
        preview={{ body: "const answer = 42;", contentType: "application/octet-stream", warnings: [] }}
      />,
    );

    expect(screen.getByText("javascript")).toBeInTheDocument();
    expect(screen.getByRole("region", { name: "最终文件内容" })).toHaveTextContent("const answer = 42;");
    view.rerender(
      <FilePreviewPage
        {...pageActions}
        fileName="rename.txt"
        preview={{ body: "const answer = 42;", contentType: "text/javascript; charset=utf-8", warnings: [] }}
      />,
    );
    expect(screen.getByText("javascript")).toBeInTheDocument();
  });

  it("keeps JSON highlighting ahead of the JavaScript filename fallback", () => {
    render(
      <FilePreviewPage
        {...pageActions}
        fileName="config.js"
        preview={{ body: "{\"enabled\":true}", contentType: "application/json", warnings: [] }}
      />,
    );

    expect(screen.getByText("json")).toBeInTheDocument();
    expect(screen.queryByText("javascript")).not.toBeInTheDocument();
  });

  it("highlights Shadowrocket .conf previews as INI", () => {
    render(
      <FilePreviewPage
        {...pageActions}
        fileName="default.conf"
        preview={{ body: "[General]\nipv6 = false", contentType: "text/plain; charset=utf-8", warnings: [] }}
      />,
    );

    expect(screen.getByText("ini")).toBeInTheDocument();
    expect(screen.getByRole("region", { name: "最终文件内容" })).toHaveTextContent("[General]");
  });

  it("keeps server warnings collapsed above the final body", async () => {
    const user = userEvent.setup();
    render(
      <FilePreviewPage
        {...pageActions}
        preview={{
          body: "processor output",
          contentType: "text/plain",
          warnings: [{ code: "processor_warning", message: "processor kept output" }],
        }}
      />,
    );

    expect(screen.getByText("1 组警告 · 1 条记录")).toBeInTheDocument();
    expect(screen.queryByText(/processor kept output/)).not.toBeInTheDocument();
    expect(screen.getByRole("region", { name: "最终文件内容" })).toHaveTextContent("processor output");

    await user.click(screen.getByRole("button", { name: "展开预览警告" }));

    expect(screen.getByText(/processor kept output/)).toBeInTheDocument();
  });

  it("keeps loading, failure, and refresh behavior", async () => {
    const user = userEvent.setup();
    const onRefresh = vi.fn();
    const onShare = vi.fn();
    const { rerender } = render(
      <FilePreviewPage backLabel="返回编辑" fileName="default.txt" pending onBack={vi.fn()} onRefresh={onRefresh} onShare={onShare} />,
    );

    expect(screen.getByRole("heading", { name: "正在处理" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "刷新文件预览" })).toBeDisabled();

    rerender(<FilePreviewPage backLabel="返回文件列表" failed fileName="default.txt" onBack={vi.fn()} onRefresh={onRefresh} onShare={onShare} />);
    expect(screen.getByRole("heading", { name: "生成失败" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "分享文件" })).toBeEnabled();

    await user.click(screen.getByRole("button", { name: "分享文件" }));
    await user.click(screen.getByRole("button", { name: "刷新文件预览" }));
    expect(onShare).toHaveBeenCalledTimes(1);
    expect(onRefresh).toHaveBeenCalledTimes(1);
  });

});

describe("FilesPage", () => {
  it("searches files and dispatches list actions", async () => {
    const user = userEvent.setup();
    const items: FileItem[] = [
      { ...files[0], displayName: "移动端配置", title: "移动端配置", description: "不应显示的描述", processorCount: 1 },
      { name: "inline.yaml", title: "inline.yaml", kind: "static", sourceType: "inline", sourceSummary: "本地" },
      { name: "summary-only.yaml", title: "summary-only.yaml", kind: "static", sourceSummary: "远程" },
      { name: "client.yaml", title: "client.yaml", kind: "mihomo", sourceType: "inline", sourceSummary: "本地" },
      { name: "client.json", title: "client.json", kind: "sing-box", sourceType: "remote", sourceSummary: "远程" },
      { name: "client.conf", title: "client.conf", kind: "shadowrocket", sourceType: "inline", sourceSummary: "本地" },
    ];
    const onCreateRemote = vi.fn();
    const onDelete = vi.fn();
    const onEdit = vi.fn();
    const onCopy = vi.fn();
    const onExport = vi.fn();
    const onPreview = vi.fn();
    const onShare = vi.fn();
    render(
      <FilesPage
        createActions={[createAction("本地", noop), createAction("远程", onCreateRemote)]}
        items={items}
        onCopy={onCopy}
        onDelete={onDelete}
        onEdit={onEdit}
        onExport={onExport}
        onPreview={onPreview}
        onShare={onShare}
      />,
    );

    expect(screen.getByText("移动端配置")).toBeInTheDocument();
    expect(screen.getByText("default.yaml")).toBeInTheDocument();
    expect(screen.queryByText("不应显示的描述")).not.toBeInTheDocument();
    const searchbox = screen.getByRole("searchbox", { name: "搜索文件" });
    fireEvent.change(searchbox, { target: { value: "移动端" } });
    expect(screen.getByText("移动端配置")).toBeInTheDocument();
    expect(screen.queryByText("inline.yaml")).not.toBeInTheDocument();
    fireEvent.change(searchbox, { target: { value: "" } });

    await user.click(screen.getByRole("button", { name: "新建文件" }));
    await user.click(await screen.findByRole("menuitem", { name: "远程" }));
    await user.click(screen.getByRole("button", { name: "编辑：移动端配置 (default.yaml)" }));
    await user.click(screen.getByRole("button", { name: "移动端配置 (default.yaml) 更多操作" }));
    expect(within(screen.getByRole("menu")).getAllByRole("menuitem").map((item) => item.textContent))
      .toEqual(["预览", "分享", "复制", "导出", "删除"]);
    expect(screen.queryByRole("menuitem", { name: "编辑" })).not.toBeInTheDocument();
    await user.click(screen.getByRole("menuitem", { name: "预览文件" }));
    await user.click(screen.getByRole("button", { name: "移动端配置 (default.yaml) 更多操作" }));
    await user.click(screen.getByRole("menuitem", { name: "分享文件" }));
    await user.click(screen.getByRole("button", { name: "移动端配置 (default.yaml) 更多操作" }));
    await user.click(screen.getByRole("menuitem", { name: "复制：移动端配置 (default.yaml)" }));
    await user.click(screen.getByRole("button", { name: "移动端配置 (default.yaml) 更多操作" }));
    await user.click(screen.getByRole("menuitem", { name: "导出：移动端配置 (default.yaml)" }));
    await user.click(screen.getByRole("button", { name: "移动端配置 (default.yaml) 更多操作" }));
    await user.click(screen.getByRole("menuitem", { name: "删除" }));

    expect(onCreateRemote).toHaveBeenCalledTimes(1);
    expect(onEdit).toHaveBeenCalledOnce();
    expect(onEdit).toHaveBeenCalledWith(items[0]);
    expect(onPreview).toHaveBeenCalledWith(items[0]);
    expect(onShare).toHaveBeenCalledWith(items[0]);
    expect(onCopy).toHaveBeenCalledWith(items[0]);
    expect(onExport).toHaveBeenCalledWith(items[0]);
    expect(onDelete).toHaveBeenCalledWith(items[0]);
  });
});
