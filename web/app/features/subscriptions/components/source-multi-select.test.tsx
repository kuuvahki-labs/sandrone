import type { SyntheticEvent } from "react";
import { act, fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import type { SubscriptionItem } from "~/features/subscriptions/model/types";
import { SubscriptionEditPage } from "~/features/subscriptions/pages/subscription-edit-page";
import {
  manySourceSubscriptions,
  noop,
  subscriptions,
} from "~/features/subscriptions/test-data";

import { SourceMultiSelect } from "./source-multi-select";

describe("SourceMultiSelect", () => {
  it("submits multiple selected source subscriptions for a collection", async () => {
    const user = userEvent.setup();
    const onSave = vi.fn((_form: FormData) => true);
    render(<SubscriptionEditPage item={subscriptions[2]} onBack={noop} onSave={onSave} sources={subscriptions} />);

    const sourcePicker = screen.getByRole("group", { name: "包含订阅" });
    await user.click(within(sourcePicker).getByRole("checkbox", { name: "warn 远程订阅 · uri-list" }));
    await user.click(within(sourcePicker).getByRole("checkbox", { name: "warn 远程订阅 · uri-list" }));
    await user.click(screen.getByRole("button", { name: "保存订阅" }));

    const saved = onSave.mock.calls[0]?.[0] as FormData;
    expect(saved.getAll("subscriptions")).toEqual(["provider", "warn"]);
  });
  it("treats an empty source picker default as an explicit empty selection", async () => {
    const user = userEvent.setup();
    const submittedSources: string[][] = [];
    const onSubmit = vi.fn((event: SyntheticEvent<HTMLFormElement, SubmitEvent>) => {
      event.preventDefault();
      submittedSources.push(new FormData(event.currentTarget).getAll("subscriptions").map(String));
    });
    render(
      <form onSubmit={onSubmit}>
        <SourceMultiSelect defaultValue={[]} subscriptions={subscriptions} />
        <button type="submit">保存</button>
      </form>,
    );

    const sourcePicker = screen.getByRole("group", { name: "包含订阅" });
    expect(within(sourcePicker).getByRole("checkbox", { name: "provider 远程订阅 · uri-list" })).not.toBeChecked();
    expect(within(sourcePicker).getByRole("checkbox", { name: "warn 远程订阅 · uri-list" })).not.toBeChecked();
    await user.click(screen.getByRole("button", { name: "保存" }));

    expect(submittedSources[0]).toEqual([]);
  });
  it("filters long source lists without dropping hidden selected subscriptions", async () => {
    const user = userEvent.setup();
    const onSave = vi.fn((_form: FormData) => true);
    const collection: SubscriptionItem = { kind: "collection", name: "many", title: "many", label: "组合订阅", status: "ready" };
    render(<SubscriptionEditPage item={collection} onBack={noop} onSave={onSave} sources={[...manySourceSubscriptions, collection]} />);

    const sourcePicker = screen.getByRole("group", { name: "包含订阅" });
    const sourceSearch = screen.getByRole("searchbox", { name: "搜索包含订阅" });
    expect(within(sourcePicker).getByText("搜索", { selector: "label" })).toBeInTheDocument();
    expect(sourceSearch).toBeInTheDocument();
    expect(sourcePicker.querySelector(".source-choice-scroll, .field-helper, .secondary-action, .ghost-action")).not.toBeInTheDocument();

    fireEvent.change(sourceSearch, { target: { value: "source-12" } });

    expect(within(sourcePicker).getByRole("checkbox", { name: "source-12 远程订阅 · uri-list" })).toBeChecked();
    expect(within(sourcePicker).queryByRole("checkbox", { name: "source-01 远程订阅 · uri-list" })).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "保存订阅" }));

    const saved = onSave.mock.calls[0]?.[0] as FormData;
    expect(saved.getAll("subscriptions")).toEqual(manySourceSubscriptions.map((item) => item.name));
  });
  it("does not report searches or no-op bulk selection as edits", async () => {
    const user = userEvent.setup();
    const onDirty = vi.fn();
    render(
      <SourceMultiSelect
        onDirty={onDirty}
        subscriptions={manySourceSubscriptions}
      />,
    );

    const sourcePicker = screen.getByRole("group", { name: "包含订阅" });
    fireEvent.change(screen.getByRole("searchbox", { name: "搜索包含订阅" }), {
      target: { value: "source-12" },
    });
    await user.click(within(sourcePicker).getByRole("button", { name: "全选" }));

    expect(onDirty).not.toHaveBeenCalled();
  });
  it("submits cleared and reselected source subscriptions from the long picker", async () => {
    const user = userEvent.setup();
    const onSave = vi.fn((_form: FormData) => true);
    const collection: SubscriptionItem = { kind: "collection", name: "many", title: "many", label: "组合订阅", status: "ready" };
    render(<SubscriptionEditPage item={collection} onBack={noop} onSave={onSave} sources={[...manySourceSubscriptions, collection]} />);

    const sourcePicker = screen.getByRole("group", { name: "包含订阅" });
    await user.click(within(sourcePicker).getByRole("button", { name: "清空" }));
    await user.click(screen.getByRole("button", { name: "保存订阅" }));

    expect((onSave.mock.calls[0]?.[0] as FormData).getAll("subscriptions")).toEqual([]);

    await user.click(within(sourcePicker).getByRole("button", { name: "全选" }));
    await user.click(screen.getByRole("button", { name: "保存订阅" }));

    expect((onSave.mock.calls[1]?.[0] as FormData).getAll("subscriptions")).toEqual(manySourceSubscriptions.map((item) => item.name));
  });
  it("preserves multiple source changes batched before a render", () => {
    const submittedSources: string[][] = [];
    const warn = { current: null as HTMLElement | null };
    const onDirty = vi.fn(() => {
      if (onDirty.mock.calls.length === 1) warn.current?.click();
    });
    const onSubmit = vi.fn((event: SyntheticEvent<HTMLFormElement, SubmitEvent>) => {
      event.preventDefault();
      submittedSources.push(new FormData(event.currentTarget).getAll("subscriptions").map(String));
    });
    render(
      <form onSubmit={onSubmit}>
        <SourceMultiSelect defaultValue={[]} onDirty={onDirty} subscriptions={subscriptions} />
        <button type="submit">保存</button>
      </form>,
    );

    const sourcePicker = screen.getByRole("group", { name: "包含订阅" });
    const provider = within(sourcePicker).getByRole("checkbox", { name: "provider 远程订阅 · uri-list" });
    warn.current = within(sourcePicker).getByRole("checkbox", { name: "warn 远程订阅 · uri-list" });
    act(() => provider.click());
    fireEvent.click(screen.getByRole("button", { name: "保存" }));

    expect(submittedSources[0]).toEqual(["provider", "warn"]);
    expect(onDirty).toHaveBeenCalledTimes(2);
  });
});
