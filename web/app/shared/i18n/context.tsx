import { createContext, type ReactNode, useContext, useEffect, useMemo, useState } from "react";

import { getLocalePreference, saveLocalePreference } from "~/shared/storage/preferences";

import { type Locale } from "./locales";
import { enUS } from "./translations/en-US";
import { type TranslationKey, zhCN } from "./translations/zh-CN";

export type { TranslationKey } from "./translations/zh-CN";

const translations: Record<Locale, Record<TranslationKey, string>> = {
  "zh-CN": zhCN,
  "en-US": enUS,
};

export type TranslationParams = Record<string, string | number>;
export type Translator = (key: TranslationKey, params?: TranslationParams) => string;

interface I18nContextValue {
  locale: Locale;
  setLocale: (locale: Locale) => void;
  t: Translator;
}

const I18nContext = createContext<I18nContextValue | null>(null);

export function I18nProvider({ children }: { children: ReactNode }) {
  const [locale, setLocaleState] = useState<Locale>(() => getLocalePreference());

  const setLocale = useMemo(() => (nextLocale: Locale) => {
    saveLocalePreference(nextLocale);
    setLocaleState(nextLocale);
  }, []);

  useEffect(() => {
    document.documentElement.lang = locale;
  }, [locale]);

  const value = useMemo<I18nContextValue>(
    () => ({
      locale,
      setLocale,
      t: createTranslator(locale),
    }),
    [locale, setLocale],
  );

  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

export function useI18n(): I18nContextValue {
  const context = useContext(I18nContext);
  if (!context) {
    const locale = getLocalePreference();
    return {
      locale,
      setLocale: saveLocalePreference,
      t: createTranslator(locale),
    };
  }
  return context;
}

export function createTranslator(locale: Locale): Translator {
  return (key, params) => translate(locale, key, params);
}

export function defaultTranslator(): Translator {
  return createTranslator(getLocalePreference());
}

export function translate(
  locale: Locale,
  key: TranslationKey,
  params?: TranslationParams,
): string {
  const template = translations[locale][key] ?? translations["en-US"][key] ?? key;
  if (!params) {
    return template;
  }
  return template.replace(/\{([a-zA-Z0-9_]+)\}/g, (match, name: string) =>
    String(params[name] ?? match),
  );
}
