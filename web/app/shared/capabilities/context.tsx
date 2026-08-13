import { createContext, useContext } from "react";

import type { UICapability } from "~/shared/api/client";

export interface UICapabilityContextValue {
  capabilities: readonly UICapability[];
  loaded: boolean;
  hasFeature: (key: string) => boolean;
  getFeature: (key: string) => UICapability | undefined;
}

const UICapabilityContext = createContext<UICapabilityContextValue>({
  capabilities: [],
  loaded: false,
  hasFeature: () => false,
  getFeature: () => undefined,
});

export const UICapabilityProvider = UICapabilityContext.Provider;

export function useUICapabilities() {
  return useContext(UICapabilityContext);
}
