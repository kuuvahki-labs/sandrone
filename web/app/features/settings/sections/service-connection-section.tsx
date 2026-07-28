import { useState } from "react";
import SaveIcon from "@mui/icons-material/Save";
import Button from "@mui/material/Button";
import Card from "@mui/material/Card";
import CardContent from "@mui/material/CardContent";
import TextField from "@mui/material/TextField";
import Typography from "@mui/material/Typography";

import { useI18n } from "~/shared/i18n/context";

interface ServiceConnectionSectionProps {
  publicBaseUrl: string;
  onSaveBaseUrl: (value: string) => void;
}

export function ServiceConnectionSection({
  publicBaseUrl,
  onSaveBaseUrl,
}: ServiceConnectionSectionProps) {
  const { t } = useI18n();
  const [baseUrl, setBaseUrl] = useState(publicBaseUrl);

  return (
    <Card component="article" variant="outlined">
      <CardContent>
        <div className="grid gap-4">
          <Typography component="h3" variant="h6">
            {t("settings.publicBaseUrl.title")}
          </Typography>
          <TextField
            fullWidth
            label="Public Base URL"
            placeholder="https://example.com"
            type="url"
            value={baseUrl}
            onChange={(event) => setBaseUrl(event.target.value)}
          />
          <Button className="justify-self-start sm:justify-self-end" aria-label={t("settings.publicBaseUrl.save")} startIcon={<SaveIcon aria-hidden />} type="button" variant="contained" onClick={() => onSaveBaseUrl(baseUrl)}>
            {t("actions.save")}
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}
