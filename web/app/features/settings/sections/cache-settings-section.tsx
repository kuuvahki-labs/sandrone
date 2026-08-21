import { useState } from "react";
import DeleteForeverOutlinedIcon from "@mui/icons-material/DeleteForeverOutlined";
import Button from "@mui/material/Button";
import Card from "@mui/material/Card";
import CardContent from "@mui/material/CardContent";
import CircularProgress from "@mui/material/CircularProgress";
import Dialog from "@mui/material/Dialog";
import DialogActions from "@mui/material/DialogActions";
import DialogContent from "@mui/material/DialogContent";
import DialogContentText from "@mui/material/DialogContentText";
import DialogTitle from "@mui/material/DialogTitle";
import Typography from "@mui/material/Typography";

import { useI18n } from "~/shared/i18n/context";

interface CacheSettingsSectionProps {
  clearing: boolean;
  onClear: () => Promise<void>;
}

export function CacheSettingsSection({ clearing, onClear }: CacheSettingsSectionProps) {
  const { t } = useI18n();
  const [confirming, setConfirming] = useState(false);

  async function confirmClear() {
    try {
      await onClear();
      setConfirming(false);
    } catch {
      // The owner reports the failure; keep the dialog open for retry.
    }
  }

  return (
    <>
      <Card component="article" variant="outlined">
        <CardContent className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <Typography component="h2" variant="h6">{t("settings.cache.title")}</Typography>
          <Button
            color="error"
            disabled={clearing}
            startIcon={clearing ? <CircularProgress size={16} /> : <DeleteForeverOutlinedIcon aria-hidden />}
            type="button"
            variant="outlined"
            onClick={() => setConfirming(true)}
          >
            {t(clearing ? "settings.cache.pending" : "settings.cache.clear")}
          </Button>
        </CardContent>
      </Card>
      <Dialog
        open={confirming}
        aria-labelledby="clear-cache-title"
        onClose={() => {
          if (!clearing) {
            setConfirming(false);
          }
        }}
      >
        <DialogTitle id="clear-cache-title">{t("settings.cache.confirmTitle")}</DialogTitle>
        <DialogContent>
          <DialogContentText>{t("settings.cache.confirmBody")}</DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button disabled={clearing} type="button" onClick={() => setConfirming(false)}>
            {t("actions.cancel")}
          </Button>
          <Button
            color="error"
            disabled={clearing}
            type="button"
            variant="contained"
            onClick={() => void confirmClear()}
          >
            {t(clearing ? "settings.cache.pending" : "settings.cache.confirmAction")}
          </Button>
        </DialogActions>
      </Dialog>
    </>
  );
}
