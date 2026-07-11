import Card from "@mui/material/Card";
import CardContent from "@mui/material/CardContent";
import CircularProgress from "@mui/material/CircularProgress";
import Typography from "@mui/material/Typography";

import { useI18n } from "~/shared/i18n/context";

import { BrandLogo } from "./brand-logo";

export function LoadingScreen() {
  const { t } = useI18n();
  return (
    <main className="flex min-h-screen items-center bg-background-default p-4">
      <Card className="mx-auto w-full max-w-[440px]">
        <CardContent>
          <div className="grid justify-items-center gap-4 text-center">
            <div className="grid justify-items-center gap-3">
              <BrandLogo size={64} src="/brand/sandrone-logo-64.png" />
              <CircularProgress aria-label={t("shell.loadingAria")} size={32} />
            </div>
            <Typography color="text.secondary" variant="overline">Sandrone</Typography>
            <Typography component="h1" variant="h4">
              {t("shell.loadingTitle")}
            </Typography>
          </div>
        </CardContent>
      </Card>
    </main>
  );
}
