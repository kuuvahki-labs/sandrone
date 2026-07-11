import { MemoryRouter } from "react-router";
import Button from "@mui/material/Button";
import Link from "@mui/material/Link";
import { useColorScheme } from "@mui/material/styles";
import { render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { loadThemePreference, saveThemePreference } from "~/shared/storage/preferences";

import { appTheme, MuiProvider } from "./material-ui";

function ThemeModeProbe() {
  const { mode } = useColorScheme();
  return <output aria-label="resolved theme mode">{mode ?? "unset"}</output>;
}

const themeWithSchemes = appTheme as typeof appTheme & {
  colorSchemes: Record<"dark" | "light", { palette: { background: { default: string }; primary: { main: string } } }>;
  cssVarPrefix: string;
};

describe("MUI theme provider", () => {
  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
    localStorage.clear();
    document.documentElement.removeAttribute("data-mui-color-scheme");
    document.documentElement.removeAttribute("data-theme-mode");
    document.documentElement.removeAttribute("data-theme-preset");
  });

  it("uses the dark MUI color scheme by default", async () => {
    render(
      <MuiProvider>
        <ThemeModeProbe />
      </MuiProvider>,
    );

    expect(loadThemePreference()).toEqual({ mode: "dark", preset: "ocean" });
    await waitFor(() => expect(screen.getByLabelText("resolved theme mode")).toHaveTextContent("dark"));
    expect(document.documentElement).toHaveAttribute("data-mui-color-scheme", "dark");
  });

  it("switches MUI color scheme when the saved theme preference changes", async () => {
    render(
      <MuiProvider>
        <ThemeModeProbe />
      </MuiProvider>,
    );

    saveThemePreference({ mode: "light", preset: "ocean" });

    await waitFor(() => expect(screen.getByLabelText("resolved theme mode")).toHaveTextContent("light"));
    expect(document.documentElement).toHaveAttribute("data-mui-color-scheme", "light");
    expect(document.documentElement).toHaveAttribute("data-theme-mode", "light");
  });

  it("lets MUI resolve system mode from the OS color preference", async () => {
    vi.stubGlobal("matchMedia", vi.fn((query: string) => ({
      matches: query === "(prefers-color-scheme: dark)",
      media: query,
      onchange: null,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn(),
    })));
    saveThemePreference({ mode: "system", preset: "ocean" });

    render(
      <MuiProvider>
        <ThemeModeProbe />
      </MuiProvider>,
    );

    await waitFor(() => expect(screen.getByLabelText("resolved theme mode")).toHaveTextContent("system"));
    expect(document.documentElement).toHaveAttribute("data-mui-color-scheme", "dark");
  });

  it("uses MUI Material Design 2 defaults instead of custom M3 component styling", () => {
    expect(themeWithSchemes.cssVarPrefix).toBe("mui");
    expect(themeWithSchemes.colorSchemes.dark.palette.primary.main).toBe("#90caf9");
    expect(themeWithSchemes.colorSchemes.light.palette.primary.main).toBe("#1976d2");
    expect(themeWithSchemes.colorSchemes.dark.palette.background.default).toBe("#121212");
    expect(themeWithSchemes.colorSchemes.light.palette.background.default).toBe("#fff");
    expect(appTheme.shape.borderRadius).toBe(4);
    expect(appTheme.components?.MuiButton?.styleOverrides).toBeUndefined();
    expect(appTheme.components?.MuiDialog?.styleOverrides).toBeUndefined();
    expect(appTheme.components?.MuiFab?.styleOverrides).toBeUndefined();
    expect(appTheme.components?.MuiBottomNavigationAction?.styleOverrides).toBeUndefined();
  });

  it("routes MUI link and button hrefs through semantic React Router links", () => {
    render(
      <MemoryRouter>
        <MuiProvider>
          <Link href="/settings">打开设置</Link>
          <Button href="/files" variant="contained">
            打开文件
          </Button>
        </MuiProvider>
      </MemoryRouter>,
    );

    expect(screen.getByRole("link", { name: "打开设置" })).toHaveAttribute("href", "/settings");
    const buttonLink = screen.getByRole("link", { name: "打开文件" });
    expect(buttonLink).toHaveAttribute("href", "/files");
    expect(buttonLink).not.toHaveAttribute("role", "button");
  });

  it("declares the MUI CSS layer before Tailwind utilities", () => {
    render(
      <MuiProvider>
        <ThemeModeProbe />
      </MuiProvider>,
    );

    expect(document.head.textContent?.replaceAll(/\s/g, "")).toContain("@layertheme,base,mui,components,utilities;");
  });
});
