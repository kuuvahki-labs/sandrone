import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it } from "vitest";

import { installDefaultFetchMock, renderApp } from "./app-routing.test-data";

describe("runtime log navigation", () => {
  beforeEach(installDefaultFetchMock);

  it("opens logs from settings and returns through the real route adapter", async () => {
    const user = userEvent.setup();
    const { router } = renderApp("/settings");
    await user.click(await screen.findByRole("button", { name: "打开程序日志" }));
    expect(await screen.findByText("暂无日志")).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/settings/logs");
    await user.click(screen.getByRole("button", { name: "返回" }));
    expect(await screen.findByRole("heading", { name: "设置" })).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/settings");
  });
});
