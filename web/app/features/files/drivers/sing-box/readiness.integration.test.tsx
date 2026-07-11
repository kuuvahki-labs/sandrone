import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, it, vi } from "vitest";

import type { ConfigNodePreviewInput } from "~/features/files/config/model/node-source";
import { FileNewPage } from "~/features/files/pages/file-new-page";
import type { ResourceOption } from "~/shared/resources/types";

const subscriptions: ResourceOption[] = [
  { name: "provider", title: "provider" },
];

afterEach(() => {
  localStorage.removeItem("sandrone.locale");
  vi.restoreAllMocks();
});

it("blocks a new sing-box file until a subscription preview has a unique named node", async () => {
  localStorage.setItem("sandrone.locale", "en-US");
  const user = userEvent.setup();
  const loadSubscriptionPreview = vi.fn().mockResolvedValue(preview(["Node 1"]));
  render(
    <FileNewPage
      loadSubscriptionPreview={loadSubscriptionPreview}
      source="sing-box"
      onBack={vi.fn()}
      onSave={vi.fn()}
      subscriptions={subscriptions}
    />,
  );

  const save = screen.getByRole("button", { name: "Save file" });
  expect(save).toBeDisabled();
  await user.click(screen.getByRole("combobox", { name: "Subscription" }));
  await user.click(await screen.findByRole("option", { name: "provider" }));
  await waitFor(() => expect(loadSubscriptionPreview).toHaveBeenCalledWith("provider"));
  await waitFor(() => expect(screen.queryAllByRole("alert").map((alert) => alert.textContent)).toEqual([]));
  expect(await screen.findByText("Loaded 1 nodes")).toBeInTheDocument();
  await waitFor(() => expect(save).toBeEnabled());
});

it("keeps save blocked when sing-box preview node tags are duplicated", async () => {
  localStorage.setItem("sandrone.locale", "en-US");
  const user = userEvent.setup();
  render(
    <FileNewPage
      loadSubscriptionPreview={vi.fn().mockResolvedValue(preview(["Node 1", "Node 1"]))}
      source="sing-box"
      onBack={vi.fn()}
      onSave={vi.fn()}
      subscriptions={subscriptions}
    />,
  );

  await user.click(screen.getByRole("combobox", { name: "Subscription" }));
  await user.click(await screen.findByRole("option", { name: "provider" }));
  expect(await screen.findByText("Loaded 2 nodes")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Save file" })).toBeDisabled();
});

it("keeps save blocked when any sing-box preview node tag is empty", async () => {
  localStorage.setItem("sandrone.locale", "en-US");
  const user = userEvent.setup();
  render(
    <FileNewPage
      loadSubscriptionPreview={vi.fn().mockResolvedValue(preview(["Node 1", ""]))}
      source="sing-box"
      onBack={vi.fn()}
      onSave={vi.fn()}
      subscriptions={subscriptions}
    />,
  );

  await user.click(screen.getByRole("combobox", { name: "Subscription" }));
  await user.click(await screen.findByRole("option", { name: "provider" }));
  expect(await screen.findByText("Loaded 1 nodes")).toBeInTheDocument();
  await user.click(screen.getByRole("button", { name: "Expand" }));
  expect(await screen.findByText("1 nodes have no name and were omitted from reference options.")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Save file" })).toBeDisabled();
});

it("blocks save when the sing-box inline base is not a JSON object", async () => {
  localStorage.setItem("sandrone.locale", "en-US");
  const user = userEvent.setup();
  render(
    <FileNewPage
      loadSubscriptionPreview={vi.fn().mockResolvedValue(preview(["Node 1"]))}
      source="sing-box"
      onBack={vi.fn()}
      onSave={vi.fn()}
      subscriptions={subscriptions}
    />,
  );

  await user.click(screen.getByRole("combobox", { name: "Subscription" }));
  await user.click(await screen.findByRole("option", { name: "provider" }));
  expect(await screen.findByText("Loaded 1 nodes")).toBeInTheDocument();
  const save = screen.getByRole("button", { name: "Save file" });
  await waitFor(() => expect(save).toBeEnabled());

  const [content] = screen.getAllByRole("textbox", { name: "Content" });
  fireEvent.change(content, { target: { value: "[]" } });
  expect(save).toBeDisabled();

  fireEvent.change(content, { target: { value: '{"log":{}}' } });
  await waitFor(() => expect(save).toBeEnabled());
});

function preview(names: string[]): ConfigNodePreviewInput {
  return {
    subscriptionName: "provider",
    nodes: names.map((name, index) => ({
      identity: `sha256:${index}`,
      after: { name, type: "ss", endpoint: `node-${index}.example:8388` },
    })),
    warnings: [],
  };
}
