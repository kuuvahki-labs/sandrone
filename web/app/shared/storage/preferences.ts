import { detectPreferredLocale, isLocale, type Locale } from "~/shared/i18n/locales";

export type ThemeMode = "system" | "light" | "dark";
export type ThemePreset = "ocean";

export interface ThemePreference {
  mode: ThemeMode;
  preset: ThemePreset;
}

const adminTokenKey = "sandrone.adminToken";
const autoLoadSubscriptionTrafficKey = "sandrone.subscriptionTraffic.autoLoad";
const localeKey = "sandrone.locale";
const publicBaseUrlKey = "sandrone.publicBaseUrl";
const themeKey = "sandrone.theme";
export const muiThemeModeStorageKey = "sandrone.theme.mode";
export const muiThemeSchemeStorageKey = "sandrone.theme.scheme";
const fallbackTheme: ThemePreference = { mode: "dark", preset: "ocean" };

export const themePreferenceChangeEvent = "sandrone:theme-preference-change";

export function saveAdminToken(token: string): void {
  browserStorage()?.setItem(adminTokenKey, token.trim());
}

export function getAdminToken(): string {
  return browserStorage()?.getItem(adminTokenKey) ?? "";
}

export function clearAdminToken(): void {
  browserStorage()?.removeItem(adminTokenKey);
}

export function getAutoLoadSubscriptionTraffic(): boolean {
  try {
    return browserStorage()?.getItem(autoLoadSubscriptionTrafficKey) === "true";
  } catch {
    return false;
  }
}

export function saveAutoLoadSubscriptionTraffic(enabled: boolean): void {
  try {
    browserStorage()?.setItem(autoLoadSubscriptionTrafficKey, String(enabled));
  } catch {
    // Ignore unavailable browser storage and retain the in-memory preference.
  }
}

export function getLocalePreference(languages?: readonly string[]): Locale {
  const stored = browserStorage()?.getItem(localeKey);
  return isLocale(stored) ? stored : detectPreferredLocale(languages);
}

export function saveLocalePreference(locale: Locale): void {
  browserStorage()?.setItem(localeKey, locale);
}

export function savePublicBaseUrl(value: string): void {
  browserStorage()?.setItem(publicBaseUrlKey, normalizeBaseUrl(value));
}

export function getPublicBaseUrl(): string {
  return browserStorage()?.getItem(publicBaseUrlKey) ?? "";
}

export function loadThemePreference(): ThemePreference {
  const raw = browserStorage()?.getItem(themeKey);
  if (!raw) {
    return themePreferenceFromMuiMode(browserStorage()?.getItem(muiThemeModeStorageKey));
  }
  try {
    const value = JSON.parse(raw) as Partial<ThemePreference>;
    return {
      mode: isThemeMode(value.mode) ? value.mode : fallbackTheme.mode,
      preset: isThemePreset(value.preset) ? value.preset : fallbackTheme.preset,
    };
  } catch {
    return fallbackTheme;
  }
}

export function saveThemePreference(preference: ThemePreference): void {
  browserStorage()?.setItem(themeKey, JSON.stringify(preference));
  browserStorage()?.setItem(muiThemeModeStorageKey, preference.mode);
  browserStorage()?.setItem(muiThemeSchemeStorageKey, resolvedMuiScheme(preference.mode));
  applyThemePreference(preference);
  dispatchThemePreferenceChange(preference);
}

export function applyThemePreference(preference: ThemePreference): void {
  if (typeof document === "undefined") {
    return;
  }
  document.documentElement.dataset.themeMode = preference.mode;
  document.documentElement.dataset.themePreset = preference.preset;
}

function dispatchThemePreferenceChange(preference: ThemePreference): void {
  if (typeof window === "undefined") {
    return;
  }
  window.dispatchEvent(new CustomEvent(themePreferenceChangeEvent, { detail: preference }));
}

function normalizeBaseUrl(value: string): string {
  return value.trim().replace(/\/+$/, "");
}

function isThemeMode(value: unknown): value is ThemeMode {
  return value === "system" || value === "light" || value === "dark";
}

function isThemePreset(value: unknown): value is ThemePreset {
  return value === "ocean";
}

function themePreferenceFromMuiMode(value: string | null | undefined): ThemePreference {
  return {
    mode: isThemeMode(value) ? value : fallbackTheme.mode,
    preset: fallbackTheme.preset,
  };
}

function resolvedMuiScheme(mode: ThemeMode): "light" | "dark" {
  if (mode === "light") {
    return "light";
  }
  return "dark";
}

function browserStorage(): Storage | null {
  return typeof localStorage === "undefined" ? null : localStorage;
}
