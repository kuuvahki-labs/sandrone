import { type ReactNode, useCallback, useEffect, useMemo, useState } from "react";

import { ApiClient, type UICapability } from "~/shared/api/client";
import { UICapabilityProvider } from "~/shared/capabilities/context";
import { useI18n } from "~/shared/i18n/context";
import { WarningPreferencesProvider } from "~/shared/resources/warning-preferences";

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
  const [uiCapabilities, setUiCapabilities] = useState<readonly UICapability[]>([]);
  const [uiCapabilitiesLoaded, setUiCapabilitiesLoaded] = useState(false);
  const reloadUiCapabilities = useCallback(async () => {
    try {
      const result = await client.getUICapabilities();
      setUiCapabilities(result.features ?? []);
      setUiCapabilitiesLoaded(true);
    } catch {
      setUiCapabilities([]);
      setUiCapabilitiesLoaded(false);
    }
  }, [client]);

  useEffect(() => {
    void reloadUiCapabilities();
  }, [reloadUiCapabilities]);

  const featureMap = useMemo(() => new Map(uiCapabilities.map((feature) => [feature.key, feature])), [uiCapabilities]);
  const hasFeature = useCallback((key: string) => featureMap.get(key)?.enabled === true, [featureMap]);
  const getFeature = useCallback((key: string) => featureMap.get(key), [featureMap]);

  function signOut() {
    preferences.signOut();
    setUiCapabilities([]);
    setUiCapabilitiesLoaded(false);
    setNeedsToken(true);
  }

  async function enterWithToken() {
    preferences.enterWithToken();
    setNeedsToken(false);
    await project.reloadSettings(true);
    await reloadUiCapabilities();
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
    uiCapabilities,
    uiCapabilitiesLoaded,
    hasFeature,
    getFeature,
    reloadUiCapabilities,
    updateSettings: project.updateSettings,
    saveBaseUrl: preferences.saveBaseUrl,
    ...deleteActions,
  };

  return (
    <SandroneContext.Provider value={value}>
      <UICapabilityProvider value={{ capabilities: uiCapabilities, loaded: uiCapabilitiesLoaded, hasFeature, getFeature }}>
        <WarningPreferencesProvider
          ignoredWarnings={project.settings.subscriptions.ignored_warnings}
          onIgnore={project.ignoreWarning}
        >
          {children}
        </WarningPreferencesProvider>
      </UICapabilityProvider>
    </SandroneContext.Provider>
  );
}
