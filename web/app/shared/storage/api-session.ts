// In-memory identity only; even logging in again with the same token starts a
// new session. Consumers use the revision, never the credential, as a cache key.
let revision = 0;
let lastToken: string | undefined;
const listeners = new Set<() => void>();

export function resetApiSession(): void {
  revision += 1;
  for (const listener of listeners) listener();
}

export function apiSession(token: string): number {
  if (lastToken !== token) {
    lastToken = token;
    resetApiSession();
  }
  return revision;
}

export function onApiSessionReset(listener: () => void): void {
  listeners.add(listener);
}
