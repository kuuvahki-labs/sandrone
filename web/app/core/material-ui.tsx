import * as React from "react";
import type { LinkProps as RouterLinkProps } from "react-router";
import { Link as RouterLink } from "react-router";
import CssBaseline from "@mui/material/CssBaseline";
import GlobalStyles from "@mui/material/GlobalStyles";
import type { LinkProps } from "@mui/material/Link";
import { createTheme, StyledEngineProvider, ThemeProvider, useColorScheme } from "@mui/material/styles";

import {
  loadThemePreference,
  muiThemeModeStorageKey,
  muiThemeSchemeStorageKey,
  type ThemeMode,
  themePreferenceChangeEvent,
} from "~/shared/storage/preferences";

const LinkBehavior = React.forwardRef<
  HTMLAnchorElement,
  Omit<RouterLinkProps, "to"> & { href: RouterLinkProps["to"] }
>((props, ref) => {
  const { href, ...other } = props;
  return <RouterLink ref={ref} to={href} {...other} />;
});

LinkBehavior.displayName = "LinkBehavior";

export const appTheme = createTheme({
  cssVariables: {
    colorSchemeSelector: "data-mui-color-scheme",
  },
  colorSchemes: {
    light: true,
    dark: true,
  },
  typography: {
    fontFamily: [
      "Roboto",
      "\"Noto Sans SC\"",
      "system-ui",
      "-apple-system",
      "BlinkMacSystemFont",
      "\"Segoe UI\"",
      "sans-serif",
    ].join(","),
    button: {
      letterSpacing: 0,
      textTransform: "none",
    },
  },
  components: {
    MuiLink: {
      defaultProps: {
        component: LinkBehavior,
      } as LinkProps,
    },
    MuiButtonBase: {
      defaultProps: {
        LinkComponent: LinkBehavior,
      },
    },
  },
});

function ColorSchemeBridge({ mode }: { mode: ThemeMode }) {
  const { setMode } = useColorScheme();

  React.useEffect(() => {
    setMode(mode);
  }, [mode, setMode]);

  return null;
}

export function MuiProvider({ children }: { children: React.ReactNode }) {
  const [themeMode, setThemeMode] = React.useState<ThemeMode>(() => loadThemePreference().mode);

  React.useEffect(() => {
    function handleThemePreferenceChange(event: Event) {
      const preference = (event as CustomEvent<ReturnType<typeof loadThemePreference>>).detail;
      setThemeMode(preference?.mode ?? loadThemePreference().mode);
    }

    window.addEventListener(themePreferenceChangeEvent, handleThemePreferenceChange);
    return () => window.removeEventListener(themePreferenceChangeEvent, handleThemePreferenceChange);
  }, []);

  return (
    <StyledEngineProvider enableCssLayer>
      <GlobalStyles styles="@layer theme, base, mui, components, utilities;" />
      <ThemeProvider
        disableTransitionOnChange
        noSsr
        colorSchemeStorageKey={muiThemeSchemeStorageKey}
        defaultMode="dark"
        modeStorageKey={muiThemeModeStorageKey}
        theme={appTheme}
      >
        <ColorSchemeBridge mode={themeMode} />
        <CssBaseline enableColorScheme />
        {children}
      </ThemeProvider>
    </StyledEngineProvider>
  );
}
