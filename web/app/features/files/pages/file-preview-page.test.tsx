import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { FilePreviewPage } from "./file-preview-page";

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
