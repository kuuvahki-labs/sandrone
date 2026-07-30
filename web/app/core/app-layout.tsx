import { useEffect, useRef } from "react";
import { Outlet, useLocation } from "react-router";
import Alert from "@mui/material/Alert";
import Button from "@mui/material/Button";
import Card from "@mui/material/Card";
import CardContent from "@mui/material/CardContent";
import Dialog from "@mui/material/Dialog";
import DialogActions from "@mui/material/DialogActions";
import DialogContent from "@mui/material/DialogContent";
import DialogContentText from "@mui/material/DialogContentText";
import DialogTitle from "@mui/material/DialogTitle";
import TextField from "@mui/material/TextField";
import Typography from "@mui/material/Typography";

import { useSandrone } from "~/core/provider/context";
import { ShareDialogProvider } from "~/features/shares/components/share-dialog-context";
import { useI18n } from "~/shared/i18n/context";
import { SnackbarStack } from "~/shared/ui/feedback";

import { BrandLogo } from "./components/brand-logo";
import { ShellFrame } from "./components/shell";

export function AppLayout() {
  const location = useLocation();
  const app = useSandrone();

  if (app.needsToken) return <AuthScreen />;

  return (
    <ShareDialogProvider
      client={app.client}
      publicBaseUrl={app.publicBaseUrl}
      showNotice={app.showNotice}
    >
      <ShellFrame activePath={location.pathname}>
        <Outlet />
        <AppOverlays />
      </ShellFrame>
    </ShareDialogProvider>
  );
}

function AuthScreen() {
  const { enterWithToken, notices, setTokenInput, tokenInput } = useSandrone();
  const { t } = useI18n();
  const tokenInputRef = useRef<HTMLInputElement | null>(null);

  useEffect(() => {
    tokenInputRef.current?.focus();
  }, []);

  return (
    <main className="flex min-h-screen items-center bg-background-default p-4">
      <Card className="mx-auto w-full max-w-[440px]">
        <CardContent>
          <form
            className="grid gap-4"
            onSubmit={(event) => {
              event.preventDefault();
              void enterWithToken();
            }}
          >
            <div aria-label={t("shell.authBrand")} className="grid justify-items-center gap-2 text-center">
              <BrandLogo size={72} src="/brand/sandrone-logo-64.png" />
              <Typography color="text.secondary" variant="overline">
                Sandrone
              </Typography>
            </div>
            <Typography className="text-center" component="h1" variant="h4">
              {t("shell.authRequired")}
            </Typography>
            <TextField
              autoComplete="current-password"
              inputRef={tokenInputRef}
              label={t("labels.adminToken")}
              type="password"
              value={tokenInput}
              onChange={(event) => setTokenInput(event.target.value)}
            />
            <Button type="submit" variant="contained">
              {t("actions.enter")}
            </Button>
            {notices.map((notice) => <Alert key={notice.id} severity={notice.severity}>{notice.message}</Alert>)}
          </form>
        </CardContent>
      </Card>
    </main>
  );
}

function AppOverlays() {
  const app = useSandrone();
  const { t } = useI18n();
  return (
    <>
      <Dialog open={Boolean(app.deleteTarget)} onClose={app.cancelDelete} aria-labelledby="delete-title">
        <DialogTitle id="delete-title">{t("shell.delete.title", { name: app.deleteTarget?.name ?? "" })}</DialogTitle>
        <DialogContent>
          <DialogContentText>{t("shell.delete.warning")}</DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button type="button" onClick={app.cancelDelete}>{t("actions.cancel")}</Button>
          <Button aria-label={t("shell.delete.action", { name: app.deleteTarget?.name ?? "" })} color="error" type="button" variant="contained" onClick={() => void app.confirmDelete()}>
            {t("actions.delete")}
          </Button>
        </DialogActions>
      </Dialog>
      <SnackbarStack notices={app.notices} />
    </>
  );
}
