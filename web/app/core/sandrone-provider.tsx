import { type ReactNode, useMemo, useState } from "react";

import { ApiClient } from "~/shared/api/client";
import { useI18n } from "~/shared/i18n/context";

import { SandroneContext } from "./provider/context";
import type { SandroneContextValue } from "./provider/types";
import { useAppPreferences } from "./provider/use-app-preferences";
import { useNotice } from "./provider/use-notice";
import { useResourceDelete } from "./provider/use-resource-delete";

export { useSandrone } from "./provider/context";
export type { DeleteTarget } from "./provider/types";

export function SandroneProvider({ children }: { children: ReactNode }) {
  const [needsToken, setNeedsToken] = useState(false);
  const client = useMemo(() => new ApiClient({ onUnauthorized: () => setNeedsToken(true) }), []);
  const { t } = useI18n();
  const { notices, showNotice } = useNotice();
  const settings = useAppPreferences({ showNotice, t });
  const deleteActions = useResourceDelete({
    client,
    showNotice,
    t,
  });

  function signOut() {
    settings.signOut();
    setNeedsToken(true);
  }

  function enterWithToken() {
    settings.enterWithToken();
    setNeedsToken(false);
  }

  const value: SandroneContextValue = {
    autoLoadSubscriptionTraffic: settings.autoLoadSubscriptionTraffic,
    client,
    enterWithToken,
    needsToken,
    notices,
    publicBaseUrl: settings.publicBaseUrl,
    setTokenInput: settings.setTokenInput,
    showNotice,
    signOut,
    themeMode: settings.themeMode,
    tokenInput: settings.tokenInput,
    updateAutoLoadSubscriptionTraffic: settings.updateAutoLoadSubscriptionTraffic,
    updateThemeMode: settings.updateThemeMode,
    saveBaseUrl: settings.saveBaseUrl,
    ...deleteActions,
  };

  return <SandroneContext.Provider value={value}>{children}</SandroneContext.Provider>;
}
