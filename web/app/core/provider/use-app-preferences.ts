import { useCallback, useState } from "react";

import { defaultTranslator, type Translator } from "~/shared/i18n/context";
import {
  clearAdminToken,
  getAdminToken,
  getPublicBaseUrl,
  saveAdminToken,
  savePublicBaseUrl,
} from "~/shared/storage/preferences";

import type { ShowNotice } from "./types";

export function useAppPreferences({ showNotice, t = defaultTranslator() }: { showNotice: ShowNotice; t?: Translator }) {
  const [initialSettings] = useState(loadInitialSettings);
  const [adminToken, setAdminTokenState] = useState(initialSettings.adminToken);
  const [tokenInput, setTokenInput] = useState(initialSettings.adminToken);
  const [publicBaseUrl, setPublicBaseUrl] = useState(initialSettings.publicBaseUrl);

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

  return {
    adminToken,
    enterWithToken,
    publicBaseUrl: effectivePublicBaseUrl(publicBaseUrl),
    saveBaseUrl,
    setTokenInput,
    signOut,
    tokenInput,
  };
}

function loadInitialSettings() {
  return {
    adminToken: getAdminToken(),
    publicBaseUrl: getPublicBaseUrl(),
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
