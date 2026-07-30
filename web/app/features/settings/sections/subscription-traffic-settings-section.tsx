import Card from "@mui/material/Card";
import CardContent from "@mui/material/CardContent";
import FormControlLabel from "@mui/material/FormControlLabel";
import Switch from "@mui/material/Switch";
import Typography from "@mui/material/Typography";

import { useI18n } from "~/shared/i18n/context";

interface SubscriptionTrafficSettingsSectionProps {
  value: boolean;
  onChange: (enabled: boolean) => void;
}

export function SubscriptionTrafficSettingsSection({
  value,
  onChange,
}: SubscriptionTrafficSettingsSectionProps) {
  const { t } = useI18n();

  return (
    <Card component="article" variant="outlined">
      <CardContent className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <Typography component="h3" variant="h6">
          {t("settings.subscriptionTraffic.title")}
        </Typography>
        <FormControlLabel
          className="m-0"
          control={(
            <Switch
              checked={value}
              onChange={(event) => onChange(event.target.checked)}
            />
          )}
          label={t("settings.subscriptionTraffic.autoLoad.label")}
        />
      </CardContent>
    </Card>
  );
}
