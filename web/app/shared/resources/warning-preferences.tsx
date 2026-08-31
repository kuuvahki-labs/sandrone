import { createContext, type ReactNode, useContext, useMemo } from "react";

import type { IgnoredWarning, PreviewWarning } from "~/shared/resources/types";
import { visiblePreviewWarnings } from "~/shared/resources/warning-groups";

interface WarningPreferencesValue {
  ignoredWarnings: readonly IgnoredWarning[];
  onIgnore?: (warning: IgnoredWarning) => Promise<unknown> | unknown;
}

const WarningPreferencesContext = createContext<WarningPreferencesValue>({
  ignoredWarnings: [],
});
const noWarnings: readonly PreviewWarning[] = [];

export function WarningPreferencesProvider({
  children,
  ignoredWarnings,
  onIgnore,
}: WarningPreferencesValue & { children: ReactNode }) {
  const value = useMemo(
    () => ({ ignoredWarnings, onIgnore }),
    [ignoredWarnings, onIgnore],
  );
  return (
    <WarningPreferencesContext.Provider value={value}>
      {children}
    </WarningPreferencesContext.Provider>
  );
}

export function useWarningPreferences(): WarningPreferencesValue {
  return useContext(WarningPreferencesContext);
}

export function useVisiblePreviewWarnings(
  warnings: readonly PreviewWarning[] | undefined,
): readonly PreviewWarning[] {
  const { ignoredWarnings } = useWarningPreferences();
  return useMemo(
    () => visiblePreviewWarnings(warnings ?? noWarnings, ignoredWarnings),
    [ignoredWarnings, warnings],
  );
}
