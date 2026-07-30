export interface EditSession {
  readonly revision: number;
  readonly persistedRevision: number;
  readonly activeSaveRevision: number | null;
}

export function createEditSession(): EditSession {
  return {
    revision: 0,
    persistedRevision: 0,
    activeSaveRevision: null,
  };
}

export function changeEditSession(session: EditSession): EditSession {
  return {
    ...session,
    revision: session.revision + 1,
  };
}

export function startEditSessionSave(session: EditSession): EditSession {
  if (isEditSessionSaving(session)) {
    throw new Error("edit session save already active");
  }
  return {
    ...session,
    activeSaveRevision: session.revision,
  };
}

export function finishEditSessionSave(
  session: EditSession,
  persisted: boolean,
): EditSession {
  if (session.activeSaveRevision === null) {
    throw new Error("edit session save is not active");
  }
  return {
    ...session,
    persistedRevision: persisted
      ? session.activeSaveRevision
      : session.persistedRevision,
    activeSaveRevision: null,
  };
}

export function isEditSessionDirty(session: EditSession): boolean {
  return session.revision !== session.persistedRevision;
}

export function isEditSessionSaving(session: EditSession): boolean {
  return session.activeSaveRevision !== null;
}
