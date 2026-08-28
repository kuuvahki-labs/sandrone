import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { I18nProvider, translate, useI18n } from "./context";

function Probe({ translationKey }: { translationKey: "nav.subscriptions" | "messages.deleted" }) {
  const { locale, setLocaleMode, t } = useI18n();
  return (
    <div>
      <p data-testid="locale">{locale}</p>
      <p data-testid="message">{translationKey === "messages.deleted" ? t(translationKey, { label: "File" }) : t(translationKey)}</p>
      <button type="button" onClick={() => setLocaleMode("en-US")}>English</button>
    </div>
  );
}

describe("I18nProvider", () => {
  it("translates config names with an explicit locale", () => {
    expect(translate("zh-CN", "files.config.outputNames.group.select")).toBe("🚀 节点选择");
    expect(translate("en-US", "files.config.outputNames.group.select")).toBe("Proxy");
    expect(translate("zh-CN", "files.config.outputNames.region.hk")).toBe("🇭🇰 香港");
    expect(translate("en-US", "files.config.outputNames.region.hk")).toBe("Hong Kong");
    expect(translate("zh-CN", "files.config.outputNames.customGroup")).toBe("自定义");
    expect(translate("en-US", "files.config.outputNames.customGroup")).toBe("Custom");
  });

  it("renders zh-CN translations by default in the test environment", () => {
    render(
      <I18nProvider>
        <Probe translationKey="nav.subscriptions" />
      </I18nProvider>,
    );

    expect(screen.getByTestId("locale")).toHaveTextContent("zh-CN");
    expect(screen.getByTestId("message")).toHaveTextContent("订阅");
  });

  it("renders English translations and interpolates params", () => {
    localStorage.setItem("sandrone.locale", "en-US");

    render(
      <I18nProvider>
        <Probe translationKey="messages.deleted" />
      </I18nProvider>,
    );

    expect(screen.getByTestId("locale")).toHaveTextContent("en-US");
    expect(screen.getByTestId("message")).toHaveTextContent("File deleted");
    expect(document.documentElement).toHaveAttribute("lang", "en-US");
  });
});
