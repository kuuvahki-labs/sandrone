import { useId, useState } from "react";
import SaveIcon from "@mui/icons-material/Save";
import StorageOutlinedIcon from "@mui/icons-material/StorageOutlined";
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
  const baseUrlLabelId = useId();

  return (
    <Card component="article" variant="outlined">
      <CardContent>
        <div className="grid gap-4">
          <div className="flex items-center gap-3">
            <StorageOutlinedIcon aria-hidden color="action" />
            <Typography component="h3" variant="h6">
              {t("settings.publicBaseUrl.title")}
            </Typography>
          </div>
          <div aria-labelledby={baseUrlLabelId} className="grid min-w-0 gap-2 sm:grid-cols-[minmax(8rem,0.45fr)_minmax(0,1fr)] sm:items-start" role="group">
            <Typography id={baseUrlLabelId}>Public Base URL</Typography>
            <div className="grid min-w-0 gap-3">
              <TextField
                fullWidth
                placeholder="https://example.com"
                slotProps={{ htmlInput: { "aria-labelledby": baseUrlLabelId } }}
                type="url"
                value={baseUrl}
                onChange={(event) => setBaseUrl(event.target.value)}
              />
              <Button className="justify-self-start sm:justify-self-end" aria-label={t("settings.publicBaseUrl.save")} startIcon={<SaveIcon aria-hidden />} type="button" variant="contained" onClick={() => onSaveBaseUrl(baseUrl)}>
                {t("actions.save")}
              </Button>
            </div>
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
