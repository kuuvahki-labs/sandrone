import { MemoryRouter } from "react-router";
import Button from "@mui/material/Button";
import Link from "@mui/material/Link";
import { useColorScheme } from "@mui/material/styles";
import { render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { loadThemePreference, saveThemePreference } from "~/shared/storage/preferences";

import { MuiProvider } from "./material-ui";

function ThemeModeProbe() {
  const { mode } = useColorScheme();
  return <output aria-label="resolved theme mode">{mode ?? "unset"}</output>;
}

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
