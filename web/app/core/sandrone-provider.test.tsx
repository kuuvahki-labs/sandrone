import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { defaultProjectSettings } from "~/features/settings/model/project-settings";
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
  "effectiveSettings",
  "enterWithToken",
  "needsToken",
  "notices",
  "publicBaseUrl",
  "reloadSettings",
  "requestDelete",
  "restartRequired",
  "saveBaseUrl",
  "setTokenInput",
  "settings",
  "settingsLoaded",
  "settingsOverrides",
  "showNotice",
  "signOut",
  "tokenInput",
  "updateSettings",
];

describe("SandroneProvider", () => {
  beforeEach(() => {
    localStorage.clear();
    vi.stubGlobal("fetch", vi.fn(async () => new Response(JSON.stringify({
      settings: defaultProjectSettings,
      effective: defaultProjectSettings,
      overrides: {},
      restart_required: [],
    }), { headers: { "content-type": "application/json" } })));
  });

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
