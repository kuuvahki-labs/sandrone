import { render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { I18nProvider } from "~/shared/i18n/context";

import { QrCodePanel } from "./qr-code";

afterEach(() => {
  vi.restoreAllMocks();
});

describe("QrCodePanel", () => {
  it("uses a compact two-module quiet zone", async () => {
    render(
      <I18nProvider>
        <QrCodePanel label="compact QR" value="fixture" />
      </I18nProvider>,
    );

    expect(await screen.findByRole("img", { name: "compact QR" })).toHaveAttribute("viewBox", "0 0 25 25");
  });

  it("falls back to selectable text supplied by its caller when content is too long", async () => {
    vi.spyOn(console, "error").mockImplementation(() => undefined);
    render(
      <I18nProvider>
        <div>
          <QrCodePanel label="oversized QR" value={"x".repeat(8_000)} />
          <code>{"x".repeat(8_000)}</code>
        </div>
      </I18nProvider>,
    );

    expect(await screen.findByRole("status")).toHaveTextContent("内容过长，无法生成二维码；仍可使用下方文本复制。");
    expect(screen.queryByRole("img", { name: "oversized QR" })).not.toBeInTheDocument();
    expect(screen.getByText("x".repeat(8_000))).toBeInTheDocument();
  });
});
