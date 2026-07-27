import { afterEach, describe, expect, it, vi } from "vitest";

import type { SubscriptionItem } from "~/features/subscriptions/model/types";
import type { ApiClient } from "~/shared/api/client";

import { createSubscriptionActions } from "./create-subscription-actions";

let originalClipboardDescriptor: PropertyDescriptor | undefined;

const subscriptions: SubscriptionItem[] = [
  { kind: "remote", name: "provider", title: "provider", label: "远程订阅", status: "ready", format: "uri-list" },
  { kind: "remote", name: "warn", title: "warn", label: "远程订阅", status: "ready", format: "uri-list" },
  { kind: "collection", name: "default", title: "default", label: "组合订阅", status: "ready" },
];

describe("subscription actions", () => {
  afterEach(() => {
    vi.useRealTimers();
    if (originalClipboardDescriptor) {
      Object.defineProperty(navigator, "clipboard", originalClipboardDescriptor);
    } else {
      Reflect.deleteProperty(navigator, "clipboard");
    }
    originalClipboardDescriptor = undefined;
  });

  it("creates remote and local subscriptions with slash-free default names", async () => {
    const { client, createSubscription, refreshResources } = setupActions();

    const remoteForm = new FormData();
    remoteForm.set("subscription_type", "remote");
    remoteForm.set("source_input", "https://www.example.com/sub");
    remoteForm.set("processors", "[]");
    await createSubscription(remoteForm);

    const localForm = new FormData();
    localForm.set("subscription_type", "local");
    localForm.set("source_input", "ss://aes-128-gcm:secret@example.com:8388#node-a");
    localForm.set("processors", "[]");
    await createSubscription(localForm);

    expect(client.createSubscription).toHaveBeenNthCalledWith(1, expect.objectContaining({ name: "example.com", type: "remote" }));
    expect(client.createSubscription).toHaveBeenNthCalledWith(2, expect.objectContaining({ name: "manual", type: "local" }));
    expect(refreshResources).toHaveBeenCalledTimes(2);
  });

  it("omits blank remote timeout so runtime defaults can apply", async () => {
    const { client, createSubscription } = setupActions();
    const form = new FormData();
    form.set("subscription_type", "remote");
    form.set("source_input", "https://www.example.com/sub");
    form.set("processors", "[]");

    await createSubscription(form);

    expect(client.createSubscription).toHaveBeenCalledWith(expect.objectContaining({
      remote: {
        url: "https://www.example.com/sub",
        user_agent: undefined,
        proxy: undefined,
        timeout_ms: undefined,
      },
    }));
  });

  it("submits remote-fetch and tri-state render cache settings", async () => {
    const { client, createSubscription } = setupActions();
    const form = new FormData();
    form.set("subscription_type", "remote");
    form.set("source_input", "https://www.example.com/sub");
    form.set("cache_ttl_seconds", "45");
    form.set("render_cache_mode", "disabled");
    form.set("processors", "[]");

    await createSubscription(form);

    expect(client.createSubscription).toHaveBeenCalledWith(expect.objectContaining({
      render_cache_ttl_seconds: 0,
      remote: expect.objectContaining({ cache_ttl_seconds: 45 }),
    }));
  });

  it("creates subscriptions with frontend-owned created and updated timestamps", async () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-06-27T01:02:03.000Z"));
    const { client, createSubscription } = setupActions();
    const form = new FormData();
    form.set("subscription_type", "remote");
    form.set("source_input", "https://www.example.com/sub");
    form.set("processors", "[]");

    await createSubscription(form);

    expect(client.createSubscription).toHaveBeenCalledWith(expect.objectContaining({
      created_at: "2026-06-27T01:02:03.000Z",
      updated_at: "2026-06-27T01:02:03.000Z",
    }));
  });

  it("creates subscriptions with display name and multiline description payload", async () => {
    const { client, createSubscription } = setupActions();
    const form = new FormData();
    form.set("subscription_type", "remote");
    form.set("display_name", "  Provider Main  ");
    form.set("description", "  daily nodes\nbackup route  ");
    form.set("source_input", "https://www.example.com/sub");
    form.set("processors", "[]");

    await createSubscription(form);

    expect(client.createSubscription).toHaveBeenCalledWith(expect.objectContaining({
      display_name: "Provider Main",
      meta: expect.objectContaining({
        description: "daily nodes\nbackup route",
        ui: "web",
      }),
    }));
  });

  it("preserves the existing subscription name when saving edits", async () => {
    const { client, navigate, refreshResources, saveSubscriptionEdit } = setupActions();
    const form = new FormData();
    form.set("name", "renamed");
    form.set("source_input", "https://example.com/sub");
    form.set("processors", "[]");

    await saveSubscriptionEdit(subscriptions[0], form);

    expect(client.createSubscription).toHaveBeenCalledWith(expect.objectContaining({ name: "provider" }));
    expect(refreshResources).toHaveBeenCalledWith();
    expect(navigate).toHaveBeenCalledWith("/subscriptions/remote/provider/edit", { replace: true });
  });

  it("preserves created_at and refreshes updated_at when saving edits", async () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-06-27T04:05:06.000Z"));
    const { client, saveSubscriptionEdit } = setupActions();
    const form = new FormData();
    form.set("source_input", "https://example.com/sub");
    form.set("processors", "[]");

    await saveSubscriptionEdit(subscriptions[0], form, {
      name: "provider",
      kind: "remote",
      createdAt: "2026-06-27T01:02:03.000Z",
      updatedAt: "2026-06-27T02:03:04.000Z",
      sourceRefs: [],
    });

    expect(client.createSubscription).toHaveBeenCalledWith(expect.objectContaining({
      created_at: "2026-06-27T01:02:03.000Z",
      updated_at: "2026-06-27T04:05:06.000Z",
    }));
  });

  it("saves subscription edits with submitted display name", async () => {
    const { client, saveSubscriptionEdit } = setupActions();
    const form = new FormData();
    form.set("display_name", "  Provider Main  ");
    form.set("source_input", "https://example.com/sub");
    form.set("processors", "[]");

    await saveSubscriptionEdit(subscriptions[0], form);

    expect(client.createSubscription).toHaveBeenCalledWith(expect.objectContaining({
      name: "provider",
      display_name: "Provider Main",
    }));
  });

  it("falls back to the existing updated_at as created_at for older subscription edits", async () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-06-27T04:05:06.000Z"));
    const { client, saveSubscriptionEdit } = setupActions();
    const form = new FormData();
    form.set("source_input", "https://example.com/sub");
    form.set("processors", "[]");

    await saveSubscriptionEdit(subscriptions[0], form, {
      name: "provider",
      kind: "remote",
      updatedAt: "2026-06-27T02:03:04.000Z",
      sourceRefs: [],
    });

    expect(client.createSubscription).toHaveBeenCalledWith(expect.objectContaining({
      created_at: "2026-06-27T02:03:04.000Z",
      updated_at: "2026-06-27T04:05:06.000Z",
    }));
  });

  it("falls back to now as created_at when saving older edits without timestamps", async () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-06-27T04:05:06.000Z"));
    const { client, saveSubscriptionEdit } = setupActions();
    const form = new FormData();
    form.set("source_input", "https://example.com/sub");
    form.set("processors", "[]");

    await saveSubscriptionEdit(subscriptions[0], form, {
      name: "provider",
      kind: "remote",
      sourceRefs: [],
    });

    expect(client.createSubscription).toHaveBeenCalledWith(expect.objectContaining({
      created_at: "2026-06-27T04:05:06.000Z",
      updated_at: "2026-06-27T04:05:06.000Z",
    }));
  });

  it("keeps the existing subscription name when saving edits as a different type", async () => {
    const { client, navigate, saveSubscriptionEdit } = setupActions();
    const form = new FormData();
    form.set("name", "renamed");
    form.set("subscription_type", "local");
    form.set("source_input", "ss://converted");
    form.set("processors", "[]");

    await saveSubscriptionEdit(subscriptions[0], form);

    const payload = client.createSubscription.mock.calls[0]?.[0] as Record<string, unknown>;
    expect(payload).toMatchObject({ name: "provider", type: "local", content: "ss://converted" });
    expect(payload).not.toHaveProperty("remote");
    expect(navigate).toHaveBeenCalledWith("/subscriptions/local/provider/edit", { replace: true });
  });

  it("rejects collection edits with no submitted sources instead of falling back to the first source", async () => {
    const { client, navigate, refreshResources, saveSubscriptionEdit, showNotice } = setupActions();
    const form = new FormData();
    form.set("name", "default");
    form.set("processors", "[]");

    await saveSubscriptionEdit(subscriptions[2], form);

    expect(client.createSubscription).not.toHaveBeenCalled();
    expect(refreshResources).not.toHaveBeenCalled();
    expect(navigate).not.toHaveBeenCalled();
    expect(showNotice).toHaveBeenCalledWith("请输入组合名称并选择至少一个包含订阅", "error");
  });

  it("copies subscription URL text to the clipboard", async () => {
    const writeText = vi.fn(async (_value: string) => undefined);
    originalClipboardDescriptor = Object.getOwnPropertyDescriptor(navigator, "clipboard");
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: { writeText },
    });
    const { copySubscriptionSource, showNotice } = setupActions();

    await copySubscriptionSource("https://example.com/sub", "url");

    expect(writeText).toHaveBeenCalledWith("https://example.com/sub");
    expect(showNotice).toHaveBeenCalledWith("已复制订阅地址");
  });

  it("copies local subscription content to the clipboard", async () => {
    const writeText = vi.fn(async (_value: string) => undefined);
    originalClipboardDescriptor = Object.getOwnPropertyDescriptor(navigator, "clipboard");
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: { writeText },
    });
    const { copySubscriptionSource, showNotice } = setupActions();

    await copySubscriptionSource("ss://example", "content");

    expect(writeText).toHaveBeenCalledWith("ss://example");
    expect(showNotice).toHaveBeenCalledWith("已复制订阅内容");
  });

});

function setupActions() {
  const client = {
    createSubscription: vi.fn(async () => ({})),
  } as unknown as ApiClient;
  const closeSheet = vi.fn();
  const navigate = vi.fn();
  const refreshResources = vi.fn(async () => undefined);
  const showNotice = vi.fn();
  return {
    client: client as ApiClient & { createSubscription: ReturnType<typeof vi.fn> },
    closeSheet,
    navigate,
    refreshResources,
    showNotice,
    ...createSubscriptionActions({
      client,
      closeSheet,
      navigate,
      refreshResources,
      showNotice,
    }),
  };
}
