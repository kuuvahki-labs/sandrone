import { useState } from "react";
import DownloadOutlinedIcon from "@mui/icons-material/DownloadOutlined";
import RestoreOutlinedIcon from "@mui/icons-material/RestoreOutlined";
import UploadFileOutlinedIcon from "@mui/icons-material/UploadFileOutlined";
import Alert from "@mui/material/Alert";
import Button from "@mui/material/Button";
import Card from "@mui/material/Card";
import CardContent from "@mui/material/CardContent";
import Dialog from "@mui/material/Dialog";
import DialogActions from "@mui/material/DialogActions";
import DialogContent from "@mui/material/DialogContent";
import DialogContentText from "@mui/material/DialogContentText";
import DialogTitle from "@mui/material/DialogTitle";
import Typography from "@mui/material/Typography";

import { useI18n } from "~/shared/i18n/context";

interface DataSettingsSectionProps {
  onDownloadBackup?: () => Promise<void>;
  onRestoreBackup?: (file: Blob) => Promise<void>;
}

export function DataSettingsSection({
  onDownloadBackup,
  onRestoreBackup,
}: DataSettingsSectionProps) {
  const { t } = useI18n();
  const [selectedBackup, setSelectedBackup] = useState<File | null>(null);
  const [backupOperation, setBackupOperation] = useState<"download" | "restore" | null>(null);
  const [confirmRestore, setConfirmRestore] = useState(false);
  const [restoreFailed, setRestoreFailed] = useState(false);

  async function downloadBackup() {
    if (!onDownloadBackup || backupOperation !== null) {
      return;
    }
    setBackupOperation("download");
    try {
      await onDownloadBackup();
    } finally {
      setBackupOperation(null);
    }
  }

  function selectBackup(file: File | undefined) {
    if (!file || backupOperation !== null) {
      return;
    }
    setSelectedBackup(file);
    setRestoreFailed(false);
    setConfirmRestore(false);
  }

  async function restoreBackup() {
    if (!onRestoreBackup || !selectedBackup || backupOperation !== null) {
      return;
    }
    setBackupOperation("restore");
    setRestoreFailed(false);
    try {
      await onRestoreBackup(selectedBackup);
      setSelectedBackup(null);
      setConfirmRestore(false);
    } catch {
      setRestoreFailed(true);
    } finally {
      setBackupOperation(null);
    }
  }

  const backupPending = backupOperation !== null;

  return (
    <>
      <Card component="article" variant="outlined">
        <CardContent>
          <div className="grid gap-4">
            <Alert role="note" severity="warning">
              {t("settings.data.warning")}
            </Alert>
            <div className="flex flex-col gap-3 sm:flex-row sm:flex-wrap">
              <Button
                disabled={backupPending || !onDownloadBackup}
                startIcon={<DownloadOutlinedIcon aria-hidden />}
                type="button"
                variant="contained"
                onClick={() => void downloadBackup().catch(() => undefined)}
              >
                {t(backupOperation === "download" ? "settings.data.downloading" : "settings.data.download")}
              </Button>
              <Button
                component="label"
                disabled={backupPending}
                startIcon={<UploadFileOutlinedIcon aria-hidden />}
                variant="outlined"
              >
                {t("settings.data.select")}
                <input
                  accept=".zip,application/zip"
                  aria-label={t("settings.data.selectFile")}
                  className="sr-only"
                  disabled={backupPending}
                  type="file"
                  onChange={(event) => {
                    selectBackup(event.currentTarget.files?.[0]);
                    event.currentTarget.value = "";
                  }}
                />
              </Button>
              <Button
                color="error"
                disabled={backupPending || !selectedBackup || !onRestoreBackup}
                startIcon={<RestoreOutlinedIcon aria-hidden />}
                type="button"
                variant="outlined"
                onClick={() => {
                  setRestoreFailed(false);
                  setConfirmRestore(true);
                }}
              >
                {t("settings.data.restore")}
              </Button>
            </div>
            {selectedBackup ? (
              <Typography aria-live="polite" className="break-words" color="text.secondary" variant="body2">
                {t("settings.data.selected", { name: selectedBackup.name })}
              </Typography>
            ) : null}
          </div>
        </CardContent>
      </Card>
      <Dialog
        open={confirmRestore}
        aria-labelledby="restore-backup-title"
        onClose={() => {
          if (backupOperation !== "restore") {
            setConfirmRestore(false);
          }
        }}
      >
        <DialogTitle id="restore-backup-title">{t("settings.data.confirmTitle")}</DialogTitle>
        <DialogContent>
          <DialogContentText>{t("settings.data.confirmBody")}</DialogContentText>
          {restoreFailed ? <Alert className="mt-3" severity="error">{t("settings.data.restoreFailed")}</Alert> : null}
        </DialogContent>
        <DialogActions>
          <Button disabled={backupOperation === "restore"} type="button" onClick={() => setConfirmRestore(false)}>
            {t("actions.cancel")}
          </Button>
          <Button
            color="error"
            disabled={backupOperation === "restore"}
            type="button"
            variant="contained"
            onClick={() => void restoreBackup()}
          >
            {t(backupOperation === "restore" ? "settings.data.restoring" : "settings.data.confirmAction")}
          </Button>
        </DialogActions>
      </Dialog>
    </>
  );
}
