import { type ReactNode, useMemo, useState } from "react";

import { ApiClient } from "~/shared/api/client";
import { useI18n } from "~/shared/i18n/context";

import { SandroneContext } from "./provider/context";
import type { SandroneContextValue } from "./provider/types";
import { useAppPreferences } from "./provider/use-app-preferences";
import { useNotice } from "./provider/use-notice";
import { useProjectSettings } from "./provider/use-project-settings";
import { useResourceDelete } from "./provider/use-resource-delete";

export { useSandrone } from "./provider/context";
export type { DeleteTarget } from "./provider/types";

export function SandroneProvider({ children }: { children: ReactNode }) {
  const [needsToken, setNeedsToken] = useState(false);
  const client = useMemo(() => new ApiClient({ onUnauthorized: () => setNeedsToken(true) }), []);
  const { setLocaleMode, t } = useI18n();
  const { notices, showNotice } = useNotice();
  const preferences = useAppPreferences({ showNotice, t });
  const project = useProjectSettings({ client, setLocaleMode, showNotice, t });
  const deleteActions = useResourceDelete({
    client,
    showNotice,
    t,
  });

  function signOut() {
    preferences.signOut();
    setNeedsToken(true);
  }

  async function enterWithToken() {
    preferences.enterWithToken();
    setNeedsToken(false);
    await project.reloadSettings(true);
  }

  const value: SandroneContextValue = {
    client,
    effectiveSettings: project.effectiveSettings,
    enterWithToken,
    needsToken,
    notices,
    publicBaseUrl: preferences.publicBaseUrl,
    reloadSettings: project.reloadSettings,
    restartRequired: project.restartRequired,
    settings: project.settings,
    settingsLoaded: project.settingsLoaded,
    settingsOverrides: project.settingsOverrides,
    setTokenInput: preferences.setTokenInput,
    showNotice,
    signOut,
    tokenInput: preferences.tokenInput,
    updateSettings: project.updateSettings,
    saveBaseUrl: preferences.saveBaseUrl,
    ...deleteActions,
  };

  return <SandroneContext.Provider value={value}>{children}</SandroneContext.Provider>;
}
