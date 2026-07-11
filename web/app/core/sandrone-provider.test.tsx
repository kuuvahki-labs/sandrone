import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it } from "vitest";

import { I18nProvider } from "~/shared/i18n/context";

import type { SandroneContextValue } from "./provider/types";
import { SandroneProvider, useSandrone } from "./sandrone-provider";

type LegacyShareKey = "closeSheet" | "createShare" | "openShareSheet" | "shareTarget" | "sheet";
type NoLegacyShareKeys = [Extract<keyof SandroneContextValue, LegacyShareKey>] extends [never] ? true : false;

const contextHasNoLegacyShareKeys: NoLegacyShareKeys = true;
const expectedContextKeys = [
  "cancelDelete",
  "client",
  "confirmDelete",
  "deleteTarget",
  "enterWithToken",
  "needsToken",
  "notices",
  "publicBaseUrl",
  "requestDelete",
  "saveBaseUrl",
  "setTokenInput",
  "showNotice",
  "signOut",
  "themeMode",
  "tokenInput",
  "updateThemeMode",
];

describe("SandroneProvider", () => {
  beforeEach(() => localStorage.clear());

  it("exposes only global auth, preference, notice, and delete contracts", () => {
    render(
      <I18nProvider>
        <SandroneProvider>
          <ContextProbe />
        </SandroneProvider>
      </I18nProvider>,
    );

    expect(contextHasNoLegacyShareKeys).toBe(true);
    expect(JSON.parse(screen.getByTestId("context-keys").textContent ?? "[]")).toEqual(expectedContextKeys);
  });
});

function ContextProbe() {
  const context = useSandrone();
  return <output data-testid="context-keys">{JSON.stringify(Object.keys(context).sort())}</output>;
}
