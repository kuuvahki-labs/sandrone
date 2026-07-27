import { act, fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, it, vi } from "vitest";

import type { RuleSetCatalogItem, RuleSetCatalogResult } from "~/features/files/model/types";

import { RuleSetCatalogDialog } from "./rule-set-catalog-dialog";

afterEach(() => {
  localStorage.removeItem("sandrone.locale");
  vi.restoreAllMocks();
});

it("loads a target once, searches locally, and renders at most 100 matches", async () => {
  localStorage.setItem("sandrone.locale", "en-US");
  const user = userEvent.setup();
  const items = Array.from({ length: 101 }, (_, index) => catalogItem(`geosite-${String(index).padStart(3, "0")}`));
  const special = catalogItem("geosite-special");
  items.push(special);
  const loadCatalog = vi.fn().mockResolvedValue({ items });
  const onAdd = vi.fn().mockReturnValue({ status: "added", ruleSets: [] });
  const onClose = vi.fn();

  render(<RuleSetCatalogDialog kind="mihomo" open loadCatalog={loadCatalog} onAdd={onAdd} onClose={onClose} />);

  expect(screen.getByRole("dialog", { name: "Rule set catalog" })).toBeInTheDocument();
  expect(await screen.findByText("geosite-000")).toBeInTheDocument();
  expect(loadCatalog).toHaveBeenCalledOnce();
  expect(loadCatalog).toHaveBeenCalledWith("mihomo");
  expect(screen.getAllByRole("listitem")).toHaveLength(100);
  expect(screen.getByText(/Showing the first 100 matches/)).toBeInTheDocument();

  const search = screen.getByRole("textbox", { name: "Search rule sets" });
  expect(screen.getByText("Search", { selector: "label" })).toBeInTheDocument();
  fireEvent.change(search, { target: { value: "special" } });
  const row = await screen.findByRole("listitem");
  expect(within(row).getByText(special.name)).toBeInTheDocument();
  expect(within(row).getByText(special.url)).toBeInTheDocument();
  expect(loadCatalog).toHaveBeenCalledOnce();

  const add = within(row).getByRole("button", { name: `Add rule set “${special.name}”` });
  expect(add).toHaveTextContent("Add");
  await user.click(add);
  expect(onAdd).toHaveBeenCalledWith({ entry: special });
  expect(onClose).toHaveBeenCalledOnce();
});

it("reuses the loaded target after closing and reopening", async () => {
  localStorage.setItem("sandrone.locale", "en-US");
  const user = userEvent.setup();
  const loadCatalog = vi.fn().mockResolvedValue({ items: [catalogItem("geosite-cn")] });
  const props = {
    kind: "mihomo" as const,
    loadCatalog,
    onAdd: vi.fn().mockReturnValue({ status: "name-conflict", existingName: "existing" }),
    onClose: vi.fn(),
  };
  const { rerender } = render(<RuleSetCatalogDialog {...props} open />);

  expect(await screen.findByText("geosite-cn")).toBeInTheDocument();
  fireEvent.change(screen.getByRole("textbox", { name: "Search rule sets" }), { target: { value: "cn" } });
  await user.click(screen.getByRole("button", { name: "Add rule set “geosite-cn”" }));
  expect(screen.getByText(/already used by another URL/)).toBeInTheDocument();
  rerender(<RuleSetCatalogDialog {...props} open={false} />);
  rerender(<RuleSetCatalogDialog {...props} open />);
  expect(await screen.findByText("geosite-cn")).toBeInTheDocument();
  expect(loadCatalog).toHaveBeenCalledOnce();
  expect(screen.getByRole("textbox", { name: "Search rule sets" })).toHaveValue("");
  expect(screen.queryByText(/already used by another URL/)).not.toBeInTheDocument();
});

it("shows loading and request error states", async () => {
  localStorage.setItem("sandrone.locale", "en-US");
  const request = deferred<RuleSetCatalogResult>();
  render(<RuleSetCatalogDialog kind="sing-box" open loadCatalog={vi.fn().mockReturnValue(request.promise)} onAdd={vi.fn()} onClose={vi.fn()} />);

  expect(screen.getByLabelText("Loading rule-set catalog")).toBeInTheDocument();
  await act(async () => {
    request.reject(new Error("Catalog snapshot unavailable"));
    await request.promise.catch(() => undefined);
  });
  expect(await screen.findByText("Catalog snapshot unavailable")).toBeInTheDocument();
  expect(screen.queryByText("No matching rule sets.")).not.toBeInTheDocument();
});

it("shows local empty and conflict states without closing", async () => {
  localStorage.setItem("sandrone.locale", "en-US");
  const user = userEvent.setup();
  const duplicate = catalogItem("geosite-duplicate");
  const conflicting = catalogItem("geosite-conflict");
  const onAdd = vi.fn()
    .mockReturnValueOnce({ status: "duplicate-url", existingName: "existing-url" })
    .mockReturnValueOnce({ status: "name-conflict", existingName: "existing-name" });
  const onClose = vi.fn();
  render(<RuleSetCatalogDialog kind="mihomo" open loadCatalog={vi.fn().mockResolvedValue({ items: [duplicate, conflicting] })} onAdd={onAdd} onClose={onClose} />);

  const rows = await screen.findAllByRole("listitem");
  await user.click(within(rows[0]).getByRole("button", { name: "Add rule set “geosite-duplicate”" }));
  expect(screen.getByText("This URL already exists as “existing-url”.")).toBeInTheDocument();
  await user.click(within(rows[1]).getByRole("button", { name: "Add rule set “geosite-conflict”" }));
  expect(screen.getByText("The name “existing-name” is already used by another URL.")).toBeInTheDocument();
  expect(onClose).not.toHaveBeenCalled();

  fireEvent.change(screen.getByRole("textbox", { name: "Search rule sets" }), { target: { value: "missing" } });
  expect(await screen.findByText("No matching rule sets.")).toBeInTheDocument();
});

function catalogItem(name: string): RuleSetCatalogItem {
  return {
    name,
    ruleKind: "domain",
    url: `https://raw.githubusercontent.com/example/catalog/main/${name}.mrs`,
  };
}

function deferred<T>(): { promise: Promise<T>; reject: (reason: unknown) => void } {
  let reject!: (reason: unknown) => void;
  const promise = new Promise<T>((_resolve, fail) => { reject = fail; });
  return { promise, reject };
}
