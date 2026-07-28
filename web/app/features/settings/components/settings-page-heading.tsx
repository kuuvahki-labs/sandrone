import ArrowBackIcon from "@mui/icons-material/ArrowBack";
import Button from "@mui/material/Button";
import Typography from "@mui/material/Typography";

import { useI18n } from "~/shared/i18n/context";

export function SettingsPageHeading({
  description,
  onBack,
  title,
}: {
  description?: string;
  onBack?: () => void;
  title: string;
}) {
  const { t } = useI18n();

  return (
    <header className="grid gap-2">
      {onBack ? (
        <Button className="w-fit px-0" startIcon={<ArrowBackIcon aria-hidden />} onClick={onBack}>
          {t("actions.back")}
        </Button>
      ) : null}
      <div>
        <Typography component="h2" variant="h4">{title}</Typography>
        {description ? <Typography color="text.secondary">{description}</Typography> : null}
      </div>
    </header>
  );
}
