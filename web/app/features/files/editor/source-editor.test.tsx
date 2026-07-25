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

  it("shows the driver base for an implicit source while preserving the empty source object", () => {
    render(
      <FileSourceEditor
        defaultValue={{}}
        inlineFallback="mixed-port: 7890"
        preserveImplicit
      />,
    );

    expect(screen.getByRole("textbox", { name: "内容" })).toHaveValue("mixed-port: 7890");
    expect(currentSource()).toEqual({});
  });

  it("keeps explicit empty remote content when switching to inline", async () => {
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
