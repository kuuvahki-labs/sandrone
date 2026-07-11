import { createContext, useContext } from "react";

import type { SandroneContextValue } from "./types";

export const SandroneContext = createContext<SandroneContextValue | null>(null);

export function useSandrone() {
  const context = useContext(SandroneContext);
  if (!context) {
    throw new Error("useSandrone must be used inside SandroneProvider");
  }
  return context;
}
