import type {
  ApiClient,
  SettingsUpdate,
  SettingsView,
} from "~/shared/api/client";

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
  effectiveSettings: SettingsView;
  enterWithToken: () => Promise<void>;
  needsToken: boolean;
  notices: Notice[];
  publicBaseUrl: string;
  reloadSettings: (fresh?: boolean) => Promise<unknown>;
  requestDelete: (target: DeleteTarget) => void;
  restartRequired: string[];
  saveBaseUrl: (value: string) => void;
  settings: SettingsView;
  settingsLoaded: boolean;
  settingsOverrides: Record<string, string>;
  setTokenInput: (value: string) => void;
  showNotice: ShowNotice;
  signOut: () => void;
  tokenInput: string;
  updateSettings: (settings: SettingsUpdate) => Promise<unknown>;
}
