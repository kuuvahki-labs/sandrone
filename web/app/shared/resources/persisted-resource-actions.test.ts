import { describe, expect, it } from "vitest";

import { createTranslator } from "~/shared/i18n/context";

import {
  persistedResourceActionBlocker,
  persistedResourceActionDisabledReason,
} from "./persisted-resource-actions";

describe("persisted resource actions", () => {
  it("prioritizes loading, then saving, then dirty state", () => {
    expect(persistedResourceActionBlocker({ dirty: false, loading: false, saving: false })).toBeNull();
    expect(persistedResourceActionBlocker({ dirty: true, loading: false, saving: false })).toBe("dirty");
    expect(persistedResourceActionBlocker({ dirty: true, loading: false, saving: true })).toBe("saving");
    expect(persistedResourceActionBlocker({ dirty: true, loading: true, saving: true })).toBe("loading");
  });

  it("returns action-specific localized reasons", () => {
    const t = createTranslator("zh-CN");

    expect(persistedResourceActionDisabledReason("preview", "dirty", t))
      .toBe("请先保存修改，再预览已保存版本");
    expect(persistedResourceActionDisabledReason("share", "saving", t))
      .toBe("保存完成后即可分享");
    expect(persistedResourceActionDisabledReason("preview", null, t)).toBeUndefined();
  });
});
