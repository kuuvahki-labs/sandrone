import type { ApiClient } from "~/shared/api/client";
import type { ThemeMode } from "~/shared/storage/preferences";

interface DeleteTargetBase {
  name: string;
  label: string;
  onDeleted?: () => Promise<void> | void;
}

export type DeleteTarget =
  | ({ kind: "subscriptions" } & DeleteTargetBase)
  | ({ kind: "files" } & DeleteTargetBase)
  | ({ kind: "shares" } & DeleteTargetBase);

export type NoticeSeverity = "success" | "error" | "warning";

export interface Notice {
  id: number;
  message: string;
  severity: NoticeSeverity;
}

export type ShowNotice = (message: string, severity?: NoticeSeverity) => void;

export interface SandroneContextValue {
  cancelDelete: () => void;
  client: ApiClient;
  confirmDelete: () => Promise<void>;
  deleteTarget: DeleteTarget | null;
  enterWithToken: () => void;
  needsToken: boolean;
  notices: Notice[];
  publicBaseUrl: string;
  requestDelete: (target: DeleteTarget) => void;
  saveBaseUrl: (value: string) => void;
  setTokenInput: (value: string) => void;
  showNotice: ShowNotice;
  signOut: () => void;
  themeMode: ThemeMode;
  tokenInput: string;
  updateThemeMode: (mode: ThemeMode) => void;
}
