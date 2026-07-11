import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";

import { FileSourceEditor } from "./source-editor";

describe("FileSourceEditor", () => {
  it("serializes remote metadata and removes it when switching to inline content", async () => {
    const user = userEvent.setup();
    render(
      <FileSourceEditor
        defaultValue={{
          type: "remote",
          remote: {
            url: "https://example.com/config.yaml",
            user_agent: "Sandrone Tests",
            proxy: "http://127.0.0.1:7890",
            timeout_ms: 2500,
          },
        }}
      />,
    );

    expect(currentSource()).toEqual({
      type: "remote",
      remote: {
        url: "https://example.com/config.yaml",
        user_agent: "Sandrone Tests",
        proxy: "http://127.0.0.1:7890",
        timeout_ms: 2500,
      },
    });

    await user.click(screen.getByRole("button", { name: "本地" }));
    await user.type(screen.getByRole("textbox", { name: "内容" }), "port: 7890");

    expect(currentSource()).toEqual({ type: "inline", content: "port: 7890" });
  });

  it("preserves an untouched local path but clears it after switching source modes", async () => {
    const user = userEvent.setup();
    render(<FileSourceEditor defaultValue={{ type: "local", path: "/config/base.yaml" }} />);

    expect(currentSource()).toEqual({ type: "local", path: "/config/base.yaml" });
    await user.click(screen.getByRole("button", { name: "远程" }));
    await user.type(screen.getByRole("textbox", { name: "远程地址" }), "https://example.com/remote.yaml");
    expect(currentSource()).toEqual({
      type: "remote",
      remote: { url: "https://example.com/remote.yaml" },
    });
  });

  it("keeps resolved empty remote content when switching to inline", async () => {
    const user = userEvent.setup();
    render(
      <FileSourceEditor
        defaultValue={{
          type: "remote",
          content: "",
          remote: { url: "https://example.com/empty.yaml" },
        }}
        inlineFallback="default: true\n"
      />,
    );

    await user.click(screen.getByRole("button", { name: "本地" }));

    expect(screen.getByRole("textbox", { name: "内容" })).toHaveValue("");
    expect(currentSource()).toEqual({ type: "inline", content: "" });
  });
});

function currentSource(): Record<string, unknown> {
  const input = document.querySelector<HTMLInputElement>('input[name="source"]');
  if (!input) throw new Error("expected serialized source input");
  return JSON.parse(input.value) as Record<string, unknown>;
}
