import { useCallback, useEffect, useState } from "react";

import { defaultTranslator, type Translator } from "~/shared/i18n/context";
import {
  applyThemePreference,
  clearAdminToken,
  getAdminToken,
  getPublicBaseUrl,
  loadThemePreference,
  saveAdminToken,
  savePublicBaseUrl,
  saveThemePreference,
  type ThemeMode,
} from "~/shared/storage/preferences";

import type { ShowNotice } from "./types";

export function useAppPreferences({ showNotice, t = defaultTranslator() }: { showNotice: ShowNotice; t?: Translator }) {
  const [initialSettings] = useState(loadInitialSettings);
  const [adminToken, setAdminTokenState] = useState(initialSettings.adminToken);
  const [tokenInput, setTokenInput] = useState(initialSettings.adminToken);
  const [publicBaseUrl, setPublicBaseUrl] = useState(initialSettings.publicBaseUrl);
  const [themeMode, setThemeMode] = useState<ThemeMode>(initialSettings.theme.mode);

  useEffect(() => {
    applyThemePreference(initialSettings.theme);
  }, [initialSettings.theme]);

  const enterWithToken = useCallback(() => {
    saveAdminToken(tokenInput);
    setAdminTokenState(tokenInput.trim());
  }, [tokenInput]);

  const signOut = useCallback(() => {
    clearAdminToken();
    setAdminTokenState("");
    setTokenInput("");
  }, []);

  const saveBaseUrl = useCallback((value: string) => {
    savePublicBaseUrl(value);
    setPublicBaseUrl(getPublicBaseUrl());
    showNotice(t("messages.settingsSaved"));
  }, [showNotice, t]);

  const updateThemeMode = useCallback((mode: ThemeMode) => {
    setThemeMode(mode);
    saveThemePreference({ mode, preset: "ocean" });
  }, []);

  return {
    adminToken,
    enterWithToken,
    publicBaseUrl: effectivePublicBaseUrl(publicBaseUrl),
    saveBaseUrl,
    setTokenInput,
    signOut,
    themeMode,
    tokenInput,
    updateThemeMode,
  };
}

function loadInitialSettings() {
  return {
    adminToken: getAdminToken(),
    publicBaseUrl: getPublicBaseUrl(),
    theme: loadThemePreference(),
  };
}

function effectivePublicBaseUrl(publicBaseUrl: string): string {
  const configured = publicBaseUrl.trim();
  if (configured) {
    return configured;
  }
  if (typeof window === "undefined") {
    return "";
  }
  return window.location.origin;
}
