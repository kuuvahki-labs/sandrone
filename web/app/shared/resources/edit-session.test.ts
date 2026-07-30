import { describe, expect, it } from "vitest";

import {
  changeEditSession,
  createEditSession,
  finishEditSessionSave,
  isEditSessionDirty,
  isEditSessionSaving,
  startEditSessionSave,
} from "./edit-session";

describe("persisted edit session", () => {
  it("starts clean and increments the revision for each change", () => {
    const clean = createEditSession();
    const changedOnce = changeEditSession(clean);
    const changedTwice = changeEditSession(changedOnce);

    expect(clean).toEqual({
      revision: 0,
      persistedRevision: 0,
      activeSaveRevision: null,
    });
    expect(isEditSessionDirty(clean)).toBe(false);
    expect(changedOnce.revision).toBe(1);
    expect(changedTwice.revision).toBe(2);
    expect(changedTwice.persistedRevision).toBe(0);
    expect(isEditSessionDirty(changedTwice)).toBe(true);
  });

  it("captures and persists the submitted revision after a successful save", () => {
    const changed = changeEditSession(createEditSession());
    const saving = startEditSessionSave(changed);
    const saved = finishEditSessionSave(saving, true);

    expect(saving.activeSaveRevision).toBe(1);
    expect(isEditSessionSaving(saving)).toBe(true);
    expect(saved).toEqual({
      revision: 1,
      persistedRevision: 1,
      activeSaveRevision: null,
    });
    expect(isEditSessionDirty(saved)).toBe(false);
    expect(isEditSessionSaving(saved)).toBe(false);
  });

  it("does not advance the persisted revision after a failed save", () => {
    const changed = changeEditSession(createEditSession());
    const saving = startEditSessionSave(changed);
    const failed = finishEditSessionSave(saving, false);

    expect(failed).toEqual({
      revision: 1,
      persistedRevision: 0,
      activeSaveRevision: null,
    });
    expect(isEditSessionDirty(failed)).toBe(true);
  });

  it("keeps edits made during a pending save dirty after success", () => {
    const submitted = changeEditSession(createEditSession());
    const saving = startEditSessionSave(submitted);
    const changedWhileSaving = changeEditSession(saving);
    const saved = finishEditSessionSave(changedWhileSaving, true);

    expect(saved).toEqual({
      revision: 2,
      persistedRevision: 1,
      activeSaveRevision: null,
    });
    expect(isEditSessionDirty(saved)).toBe(true);
  });

  it("rejects a second save start while a save is active", () => {
    const saving = startEditSessionSave(changeEditSession(createEditSession()));

    expect(() => startEditSessionSave(saving)).toThrow();
  });
});
