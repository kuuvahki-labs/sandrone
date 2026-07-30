import { fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import type { FileItem } from "~/features/files/model/types";
import { createAction, files, noop } from "~/features/files/test-data";

import { FilePreviewPage } from "./file-preview-page";
import { FilesPage } from "./files-page";

const pageActions = {
  fileName: "default.txt",
  onBack: vi.fn(),
  onRefresh: vi.fn(),
};

describe("FilePreviewPage", () => {
  it.each([
    ["JSON", "application/json", "{\"future\":true}", "future"],
    ["YAML", "application/yaml", "rules:\n  - allow\n", "allow"],
    ["plain text", "text/plain", "plain final output", "plain final output"],
    ["empty", "application/octet-stream", "", ""],
  ])("shows the final %s body directly", (_, contentType, body, expected) => {
    render(
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
    expect(screen.queryByRole("tab")).not.toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it.each([
    ["rename.JS", "application/octet-stream"],
    ["rename.txt", "text/javascript; charset=utf-8"],
  ])("highlights %s with %s as JavaScript", (fileName, contentType) => {
    render(
      <FilePreviewPage
        {...pageActions}
        fileName={fileName}
        preview={{ body: "const answer = 42;", contentType, warnings: [] }}
      />,
    );

    expect(screen.getByText("javascript")).toBeInTheDocument();
    expect(screen.getByRole("region", { name: "最终文件内容" })).toHaveTextContent("const answer = 42;");
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

  it("fills the viewport remainder when final source is available", () => {
    render(
      <FilePreviewPage
        {...pageActions}
        preview={{ body: "line 1\nline 2", contentType: "text/plain", warnings: [] }}
      />,
    );

    const page = screen.getByRole("heading", { name: "文件预览" }).closest("section");
    const block = screen.getByRole("region", { name: "最终文件内容" });
    expect(page).toHaveClass(
      "flex",
      "h-[calc(100dvh-2.5rem)]",
      "min-[820px]:h-[calc(100dvh-3rem)]",
      "min-h-0",
      "flex-col",
    );
    expect(block).toHaveClass("flex", "min-h-0", "flex-1", "flex-col");
  });

  it("keeps server warnings above the final body", () => {
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

    expect(screen.getByText(/processor kept output/)).toBeInTheDocument();
    expect(screen.getByRole("region", { name: "最终文件内容" })).toHaveTextContent("processor output");
  });

  it("keeps loading, failure, and refresh behavior", async () => {
    const user = userEvent.setup();
    const onRefresh = vi.fn();
    const { rerender } = render(
      <FilePreviewPage fileName="default.txt" pending onBack={vi.fn()} onRefresh={onRefresh} />,
    );

    expect(screen.getByRole("heading", { name: "正在生成" })).toBeInTheDocument();
    const page = screen.getByRole("heading", { name: "文件预览" }).closest("section");
    expect(page).toHaveClass("grid");
    expect(page).not.toHaveClass("h-[calc(100dvh-2.5rem)]");
    expect(screen.getByRole("button", { name: "刷新文件预览" })).toBeDisabled();

    rerender(<FilePreviewPage failed fileName="default.txt" onBack={vi.fn()} onRefresh={onRefresh} />);
    expect(screen.getByRole("heading", { name: "生成失败" })).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "刷新文件预览" }));
    expect(onRefresh).toHaveBeenCalledTimes(1);
  });
});

describe("FilesPage", () => {
  it("renders the file summary, search, source icons, and list actions", async () => {
    const user = userEvent.setup();
    const items: FileItem[] = [
      { ...files[0], displayName: "移动端配置", title: "移动端配置", processorCount: 1 },
      { name: "inline.yaml", title: "inline.yaml", kind: "static", sourceType: "inline", sourceSummary: "本地" },
      { name: "summary-only.yaml", title: "summary-only.yaml", kind: "static", sourceSummary: "远程" },
      { name: "client.yaml", title: "client.yaml", kind: "mihomo", sourceType: "inline", sourceSummary: "本地" },
      { name: "client.json", title: "client.json", kind: "sing-box", sourceType: "remote", sourceSummary: "远程" },
      { name: "client.conf", title: "client.conf", kind: "shadowrocket", sourceType: "inline", sourceSummary: "本地" },
    ];
    const onCreateRemote = vi.fn();
    const onDelete = vi.fn();
    const onEdit = vi.fn();
    const onShare = vi.fn();
    render(
      <FilesPage
        createActions={[createAction("本地", noop), createAction("远程", onCreateRemote)]}
        items={items}
        onDelete={onDelete}
        onEdit={onEdit}
        onShare={onShare}
      />,
    );

    expect(screen.getByRole("heading", { name: "我的文件" })).toBeInTheDocument();
    const summary = screen.getByLabelText("文件摘要");
    expect(within(summary).getByText("总数")).toBeInTheDocument();
    expect(within(summary).getByText("本地")).toBeInTheDocument();
    expect(within(summary).getByText("远程")).toBeInTheDocument();
    expect(within(summary).getByText("配置文件")).toBeInTheDocument();
    const list = screen.getByRole("list", { name: "文件列表" });
    expect(iconForListItem(list, "编辑：default.yaml", "CloudOutlinedIcon")).toBeInTheDocument();
    expect(iconForListItem(list, "编辑：inline.yaml", "DescriptionOutlinedIcon")).toBeInTheDocument();
    expect(iconForListItem(list, "编辑：summary-only.yaml", "CloudOutlinedIcon")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "编辑：client.yaml" }).querySelector('img[src="/brand/clients/mihomo.webp"]')).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "编辑：client.json" }).querySelector('img[src="/brand/clients/sing-box.svg"]')).toBeInTheDocument();
    expect(iconForListItem(list, "编辑：client.conf", "RocketLaunchOutlinedIcon")).toBeInTheDocument();

    const searchbox = screen.getByRole("searchbox", { name: "搜索文件" });
    expect(screen.getByText("搜索", { selector: "label" })).toBeInTheDocument();
    fireEvent.change(searchbox, { target: { value: "移动端" } });
    expect(screen.getByText("移动端配置")).toBeInTheDocument();
    expect(screen.queryByText("inline.yaml")).not.toBeInTheDocument();
    fireEvent.change(searchbox, { target: { value: "" } });

    await user.click(screen.getByRole("button", { name: "新建文件" }));
    await user.click(await screen.findByRole("menuitem", { name: "远程" }));
    await user.click(screen.getByRole("button", { name: "编辑：default.yaml" }));
    await user.click(screen.getByRole("button", { name: "default.yaml 更多操作" }));
    await user.click(screen.getByRole("menuitem", { name: "编辑" }));
    await user.click(screen.getByRole("button", { name: "default.yaml 更多操作" }));
    await user.click(screen.getByRole("menuitem", { name: "分享" }));
    await user.click(screen.getByRole("button", { name: "default.yaml 更多操作" }));
    await user.click(screen.getByRole("menuitem", { name: "删除" }));

    expect(onCreateRemote).toHaveBeenCalledTimes(1);
    expect(onEdit).toHaveBeenCalledTimes(2);
    expect(onEdit).toHaveBeenNthCalledWith(1, items[0]);
    expect(onEdit).toHaveBeenNthCalledWith(2, items[0]);
    expect(onShare).toHaveBeenCalledWith(items[0]);
    expect(onDelete).toHaveBeenCalledWith(items[0]);
  });
});

function iconForListItem(list: HTMLElement, actionName: string, iconTestId: string): Element | null {
  const item = within(list).getByRole("button", { name: actionName }).closest("li");
  expect(item).not.toBeNull();
  return item?.querySelector(`[data-testid="${iconTestId}"]`) ?? null;
}
