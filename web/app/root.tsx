import "./app.css";

import {
  isRouteErrorResponse,
  Links,
  Meta,
  Scripts,
  ScrollRestoration,
} from "react-router";
import Card from "@mui/material/Card";
import CardContent from "@mui/material/CardContent";
import InitColorSchemeScript from "@mui/material/InitColorSchemeScript";
import Typography from "@mui/material/Typography";

import type { Route } from "./+types/root";
import { AppLayout } from "./core/app-layout";
import { MuiProvider } from "./core/material-ui";
import { SandroneProvider } from "./core/sandrone-provider";
import { I18nProvider, translate } from "./shared/i18n/context";
import { getLocalePreference, muiThemeModeStorageKey, muiThemeSchemeStorageKey } from "./shared/storage/preferences";

export const links: Route.LinksFunction = () => [
  { rel: "icon", href: "/brand/favicon.ico", sizes: "any" },
  { rel: "icon", href: "/brand/sandrone-logo-32.png", sizes: "32x32", type: "image/png" },
  { rel: "icon", href: "/brand/sandrone-logo-48.png", sizes: "48x48", type: "image/png" },
];

export function Layout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en-US" suppressHydrationWarning>
      <head>
        <meta charSet="utf-8" />
        <meta name="viewport" content="width=device-width, initial-scale=1" />
        <title>Sandrone</title>
        <meta name="description" content="Sandrone subscription management app" />
        <Meta />
        <Links />
      </head>
      <body>
        <InitColorSchemeScript
          attribute="data-mui-color-scheme"
          colorSchemeStorageKey={muiThemeSchemeStorageKey}
          defaultMode="dark"
          modeStorageKey={muiThemeModeStorageKey}
        />
        {children}
        <ScrollRestoration />
        <Scripts />
      </body>
    </html>
  );
}

export default function App() {
  return (
    <MuiProvider>
      <I18nProvider>
        <SandroneProvider>
          <AppLayout />
        </SandroneProvider>
      </I18nProvider>
    </MuiProvider>
  );
}

export function ErrorBoundary({ error }: Route.ErrorBoundaryProps) {
  const locale = getLocalePreference();
  let message = translate(locale, "errors.pageUnavailable");
  let details = translate(locale, "errors.unexpected");
  let stack: string | undefined;

  if (isRouteErrorResponse(error)) {
    message = error.status === 404 ? "404" : translate(locale, "errors.requestFailed");
    details =
      error.status === 404
        ? translate(locale, "errors.notFound")
        : error.statusText || details;
  } else if (import.meta.env.DEV && error && error instanceof Error) {
    details = error.message;
    stack = error.stack;
  }

  return (
    <main className="flex min-h-screen items-center bg-background-default p-4">
      <Card className="mx-auto w-full max-w-[560px]">
        <CardContent>
          <Typography component="h1" gutterBottom variant="h4">
            {message}
          </Typography>
          <Typography className="mb-4" color="text.secondary" component="p">
            {details}
          </Typography>
          {stack && (
            <pre>
              <code>{stack}</code>
            </pre>
          )}
        </CardContent>
      </Card>
    </main>
  );
}
