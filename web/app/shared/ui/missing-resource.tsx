import Button from "@mui/material/Button";
import Card from "@mui/material/Card";
import CardContent from "@mui/material/CardContent";
import Typography from "@mui/material/Typography";

import { useI18n } from "~/shared/i18n/context";

export function MissingResource({ onBack, title }: { onBack: () => void; title: string }) {
  const { t } = useI18n();
  return (
    <section className="grid gap-6">
      <Card component="article" variant="outlined">
        <CardContent>
          <div className="grid justify-items-start gap-4">
            <div>
              <Typography component="h2" variant="h5">
                {title}
              </Typography>
              <Typography color="text.secondary">{t("missing.description")}</Typography>
            </div>
            <Button type="button" variant="contained" onClick={onBack}>
              {t("actions.backToList")}
            </Button>
          </div>
        </CardContent>
      </Card>
    </section>
  );
}
