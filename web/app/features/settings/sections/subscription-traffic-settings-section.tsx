import Card from "@mui/material/Card";
import CardContent from "@mui/material/CardContent";
import FormControlLabel from "@mui/material/FormControlLabel";
import Switch from "@mui/material/Switch";
import Typography from "@mui/material/Typography";

import { useI18n } from "~/shared/i18n/context";

interface SubscriptionTrafficSettingsSectionProps {
  autoLoadSubscriptionTraffic: boolean;
  onAutoLoadSubscriptionTraffic: (enabled: boolean) => void;
}

export function SubscriptionTrafficSettingsSection({
  autoLoadSubscriptionTraffic,
  onAutoLoadSubscriptionTraffic,
}: SubscriptionTrafficSettingsSectionProps) {
  const { t } = useI18n();

  return (
    <Card component="article" variant="outlined">
      <CardContent>
        <div className="grid gap-3">
          <Typography component="h3" variant="h6">
            {t("settings.subscriptionTraffic.title")}
          </Typography>
          <div>
            <FormControlLabel
              control={(
                <Switch
                  checked={autoLoadSubscriptionTraffic}
                  onChange={(event) => onAutoLoadSubscriptionTraffic(event.target.checked)}
                />
              )}
              label={t("settings.subscriptionTraffic.autoLoad.label")}
            />
            <Typography color="text.secondary" variant="body2">
              {t("settings.subscriptionTraffic.autoLoad.description")}
            </Typography>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
