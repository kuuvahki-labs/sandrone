import Card from "@mui/material/Card";
import CardContent from "@mui/material/CardContent";
import Typography from "@mui/material/Typography";

import { useI18n } from "~/shared/i18n/context";

export function PreviewPendingStatus({ elapsedSeconds }: { elapsedSeconds: number }) {
  const { t } = useI18n();
  const message = elapsedSeconds > 0
    ? t("preview.processingElapsed", { seconds: elapsedSeconds })
    : t("preview.processing");

  return (
    <Card component="article" variant="outlined">
      <CardContent>
        <div aria-atomic="true" role="status">
          <Typography component="h3" variant="h6">
            {message}
          </Typography>
        </div>
      </CardContent>
    </Card>
  );
}
