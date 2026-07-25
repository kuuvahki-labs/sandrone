import { act, renderHook, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import type { FileDetail } from "~/features/files/model/types";
import { type ApiClient, ApiError } from "~/shared/api/client";
import type { Translator } from "~/shared/i18n/context";

import { useFileDetailsResource, useFileResources } from "./use-file-resources";

const t: Translator = (key) => key === "errors.fileDefinitionLoadFailed"
  ? "file definition fallback"
  : key === "errors.filePreviewFailed"
    ? "file preview fallback"
    : key;
const ignoreNotice = () => undefined;

describe("file resource data", () => {
  it("decodes and sorts file resources by the default resource order", async () => {
    const client = apiClient({
      listFiles: vi.fn().mockResolvedValue({ items: [
        { name: "older.yaml", target: "static", created_at: "2026-01-01T00:00:00Z" },
        { name: "newer.yaml", target: "static", created_at: "2026-02-01T00:00:00Z" },
      ] }),
    });
    const { result } = renderHook(() => useFileResources({ client, showNotice: ignoreNotice, t }));

    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.items.map((item) => item.name)).toEqual(["newer.yaml", "older.yaml"]);
  });

  it("loads complete inline details from the spec without requesting source content", async () => {
    const spec = deferred<unknown>();
    const getFileSpec = vi.fn(() => spec.promise);
    const getFileSource = vi.fn();
    const client = apiClient({ getFileSource, getFileSpec });
    const { result } = renderHook(() => useFileDetailsResource({ client, showNotice: ignoreNotice, t }));

    let detailPromise!: ReturnType<typeof result.current.loadFileDetail>;
    act(() => {
      detailPromise = result.current.loadFileDetail("profile.yaml");
    });
    expect(getFileSpec).toHaveBeenCalledWith("profile.yaml");
    expect(getFileSource).not.toHaveBeenCalled();

    spec.resolve({
      name: "profile.yaml",
      kind: "static",
      source: { type: "inline", content: "proxies: []\n" },
    });
    let detail: FileDetail | null = null;
    await act(async () => {
      detail = await detailPromise;
    });

    expect(detail).toMatchObject({
      name: "profile.yaml",
      source: {
        type: "inline",
        content: "proxies: []\n",
      },
    });
    expect(result.current.fileDetails["profile.yaml"]).toEqual(detail);
  });

  it("keeps an implicit typed source distinct from explicit empty inline content", async () => {
    const client = apiClient({
      getFileSpec: vi.fn().mockResolvedValue({ name: "legacy.txt", kind: "static", source: {} }),
      getFileSource: vi.fn(),
    });
    const { result } = renderHook(() => useFileDetailsResource({ client, showNotice: ignoreNotice, t }));

    await act(async () => {
      await result.current.loadFileDetail("legacy.txt");
    });

    expect(result.current.fileDetails["legacy.txt"]?.source).toEqual({});
    expect(client.getFileSource).not.toHaveBeenCalled();
  });

  it("decodes file previews", async () => {
    const client = apiClient({
      previewFile: vi.fn().mockResolvedValue({
        content_type: "application/yaml",
        body: "proxies: []\n",
        warnings: [{ code: "warning", message: "kept" }],
      }),
    });
    const { result } = renderHook(() => useFileDetailsResource({ client, showNotice: ignoreNotice, t }));

    let preview = null;
    await act(async () => {
      preview = await result.current.loadFilePreview("profile.yaml");
    });

    expect(preview).toEqual(expect.objectContaining({
      contentType: "application/yaml",
      body: "proxies: []\n",
      warnings: [expect.objectContaining({ code: "warning", message: "kept" })],
    }));
  });

  it("keeps 401 detail failures silent and reports non-Error preview failures with fallback copy", async () => {
    const showNotice = vi.fn();
    const client = apiClient({
      getFileSpec: vi.fn().mockRejectedValue(new ApiError(401, "unauthorized", "unauthorized")),
      previewFile: vi.fn().mockRejectedValue("offline"),
    });
    const { result } = renderHook(() => useFileDetailsResource({ client, showNotice, t }));

    let detail = undefined;
    await act(async () => {
      detail = await result.current.loadFileDetail("private.yaml");
    });
    expect(detail).toBeNull();
    expect(showNotice).not.toHaveBeenCalled();

    let preview = undefined;
    await act(async () => {
      preview = await result.current.loadFilePreview("private.yaml");
    });
    expect(preview).toBeNull();
    expect(showNotice).toHaveBeenCalledOnce();
    expect(showNotice).toHaveBeenCalledWith("file preview fallback", "error");
  });
});

function apiClient(methods: Partial<ApiClient>): ApiClient {
  return methods as ApiClient;
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason: unknown) => void;
  const promise = new Promise<T>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise;
    reject = rejectPromise;
  });
  return { promise, reject, resolve };
}
