import {
  createContext,
  type ReactNode,
  useCallback,
  useContext,
  useMemo,
  useRef,
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
  publicBaseUrl: string;
  showNotice: (message: string, severity?: "success" | "error" | "warning") => void;
}

const ShareDialogContext = createContext<ShareDialogContextValue | null>(null);

export function ShareDialogProvider({ children, client, publicBaseUrl, showNotice }: ShareDialogProviderProps) {
  const [dialog, setDialog] = useState<{ id: number; target: ShareTarget } | null>(null);
  const nextDialogId = useRef(0);
  const { t } = useI18n();
  const close = useCallback(() => setDialog(null), []);
  const open = useCallback((target: ShareTarget) => {
    nextDialogId.current += 1;
    setDialog({ id: nextDialogId.current, target });
  }, []);
  const { copyShare, createShare } = useMemo(() => createShareActions({
    client,
    publicBaseUrl,
    showNotice,
    t,
  }), [client, publicBaseUrl, showNotice, t]);
  const value = useMemo(() => ({ close, open }), [close, open]);

  return (
    <ShareDialogContext.Provider value={value}>
      {children}
      {dialog ? (
        <ShareDialog
          key={dialog.id}
          target={dialog.target}
          onClose={close}
          onCopy={copyShare}
          onSubmit={createShare}
        />
      ) : null}
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
