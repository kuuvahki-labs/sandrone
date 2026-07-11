import {
  createContext,
  type ReactNode,
  useCallback,
  useContext,
  useMemo,
  useState,
} from "react";

import { createShareActions } from "~/features/shares/data/create-share-actions";
import type { ShareTarget } from "~/features/shares/model/types";
import type { ApiClient } from "~/shared/api/client";
import { useI18n } from "~/shared/i18n/context";

import { ShareDialog } from "./share-dialog";

interface ShareDialogContextValue {
  close: () => void;
  open: (target: ShareTarget) => void;
}

interface ShareDialogProviderProps {
  children: ReactNode;
  client: ApiClient;
  showNotice: (message: string, severity?: "success" | "error" | "warning") => void;
}

const ShareDialogContext = createContext<ShareDialogContextValue | null>(null);

export function ShareDialogProvider({ children, client, showNotice }: ShareDialogProviderProps) {
  const [target, setTarget] = useState<ShareTarget | null>(null);
  const { t } = useI18n();
  const close = useCallback(() => setTarget(null), []);
  const open = useCallback((nextTarget: ShareTarget) => setTarget(nextTarget), []);
  const { createShare } = useMemo(() => createShareActions({
    client,
    closeSheet: close,
    showNotice,
    t,
  }), [client, close, showNotice, t]);
  const value = useMemo(() => ({ close, open }), [close, open]);

  return (
    <ShareDialogContext.Provider value={value}>
      {children}
      {target ? <ShareDialog target={target} onClose={close} onSubmit={createShare} /> : null}
    </ShareDialogContext.Provider>
  );
}

export function useShareDialog() {
  const context = useContext(ShareDialogContext);
  if (!context) {
    throw new Error("useShareDialog must be used inside ShareDialogProvider");
  }
  return context;
}
