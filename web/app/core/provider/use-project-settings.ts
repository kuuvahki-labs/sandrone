import { useCallback, useEffect, useRef, useState } from "react";

import {
  completeProjectSettings,
  defaultProjectSettings,
  defaultSettingsEnvelope,
  optimisticSettingsEnvelope,
  settingsUpdateFromView,
} from "~/features/settings/model/project-settings";
import type {
  ApiClient,
  SettingsEnvelope,
  SettingsUpdate,
} from "~/shared/api/client";
import type { Translator } from "~/shared/i18n/context";
import type { IgnoredWarning } from "~/shared/resources/types";
import { warningIgnoreKey } from "~/shared/resources/warning-groups";
import {
  getLocaleModePreference,
  loadThemePreference,
  type LocaleMode,
  saveLocalePreference,
  saveThemePreference,
} from "~/shared/storage/preferences";

import type { ShowNotice } from "./types";

interface UseProjectSettingsOptions {
  client: ApiClient;
  setLocaleMode: (mode: LocaleMode) => void;
  showNotice: ShowNotice;
  t: Translator;
}

export function useProjectSettings({
  client,
  setLocaleMode,
  showNotice,
  t,
}: UseProjectSettingsOptions) {
  const [envelope, setEnvelope] = useState<SettingsEnvelope>(initialEnvelope);
  const [settingsLoaded, setSettingsLoaded] = useState(false);
  const requestGeneration = useRef(0);
  const envelopeRef = useRef(envelope);
  const showNoticeRef = useRef(showNotice);
  const translatorRef = useRef(t);

  useEffect(() => {
    showNoticeRef.current = showNotice;
    translatorRef.current = t;
  }, [showNotice, t]);

  const applyAppearance = useCallback((next: SettingsEnvelope) => {
    const appearance = next.effective.appearance;
    saveThemePreference({ mode: appearance.theme_mode, preset: "ocean" });
    saveLocalePreference(appearance.locale);
    setLocaleMode(appearance.locale);
  }, [setLocaleMode]);

  const reloadSettings = useCallback(async (fresh = false) => {
    const generation = ++requestGeneration.current;
    try {
      const loaded = normalizeEnvelope(await client.getSettings({ fresh }));
      if (requestGeneration.current === generation) {
        envelopeRef.current = loaded;
        setEnvelope(loaded);
        setSettingsLoaded(true);
        applyAppearance(loaded);
      }
      return loaded;
    } catch (error) {
      if (requestGeneration.current === generation) {
        setSettingsLoaded(false);
        showNoticeRef.current(translatorRef.current("errors.settingsLoadFailed"), "error");
      }
      throw error;
    }
  }, [applyAppearance, client]);

  useEffect(() => {
    void reloadSettings().catch(() => undefined);
    return () => {
      requestGeneration.current += 1;
    };
  }, [reloadSettings]);

  const updateSettings = useCallback(async (update: SettingsUpdate) => {
    const generation = ++requestGeneration.current;
    const previous = envelopeRef.current;
    const optimistic = normalizeEnvelope(optimisticSettingsEnvelope(previous, update));
    envelopeRef.current = optimistic;
    setEnvelope(optimistic);
    applyAppearance(optimistic);
    try {
      const saved = normalizeEnvelope(await client.updateSettings(update));
      if (requestGeneration.current === generation) {
        envelopeRef.current = saved;
        setEnvelope(saved);
        setSettingsLoaded(true);
        applyAppearance(saved);
        showNoticeRef.current(translatorRef.current("messages.settingsSaved"));
      }
      return saved;
    } catch (error) {
      if (requestGeneration.current === generation) {
        envelopeRef.current = previous;
        setEnvelope(previous);
        applyAppearance(previous);
        showNoticeRef.current(translatorRef.current("errors.requestFailed"), "error");
      }
      throw error;
    }
  }, [applyAppearance, client]);

  const ignoreWarning = useCallback(async (warning: IgnoredWarning) => {
    const current = envelopeRef.current.settings;
    const ignoredWarnings = current.subscriptions.ignored_warnings;
    const key = warningIgnoreKey(warning);
    if (ignoredWarnings.some((item) => warningIgnoreKey(item) === key)) return envelopeRef.current;
    return updateSettings(settingsUpdateFromView({
      ...current,
      subscriptions: {
        ...current.subscriptions,
        ignored_warnings: [...ignoredWarnings, warning],
      },
    }));
  }, [updateSettings]);

  return {
    settings: envelope.settings,
    effectiveSettings: envelope.effective,
    settingsLoaded,
    settingsOverrides: envelope.overrides,
    restartRequired: envelope.restart_required,
    ignoreWarning,
    reloadSettings,
    updateSettings,
  };
}

function initialEnvelope(): SettingsEnvelope {
  const cached = completeProjectSettings({
    ...defaultProjectSettings,
    appearance: {
      theme_mode: loadThemePreference().mode,
      locale: getLocaleModePreference(),
    },
    subscriptions: {
      auto_load_traffic: false,
      ignored_warnings: [],
    },
  });
  return defaultSettingsEnvelope(cached);
}

function normalizeEnvelope(value: SettingsEnvelope): SettingsEnvelope {
  return {
    settings: completeProjectSettings(value.settings),
    effective: completeProjectSettings(value.effective),
    overrides: Object.fromEntries(Object.entries(value.overrides ?? {}).sort(([left], [right]) => left.localeCompare(right))),
    restart_required: [...(value.restart_required ?? [])].sort(),
  };
}
