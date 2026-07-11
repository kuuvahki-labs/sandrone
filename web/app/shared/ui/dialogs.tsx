import { type ReactNode, useId } from "react";
import Button from "@mui/material/Button";
import Dialog from "@mui/material/Dialog";
import DialogActions from "@mui/material/DialogActions";
import DialogContent from "@mui/material/DialogContent";
import DialogContentText from "@mui/material/DialogContentText";
import DialogTitle from "@mui/material/DialogTitle";

import { useI18n } from "~/shared/i18n/context";

export function AppDialog({ children, onClose, title }: { children: ReactNode; onClose: () => void; title: string }) {
  const titleId = useId();
  return (
    <Dialog
      fullWidth
      open
      aria-labelledby={titleId}
      maxWidth="sm"
      scroll="paper"
      onClose={onClose}
    >
      <DialogTitle id={titleId}>{title}</DialogTitle>
      <DialogContent dividers>{children}</DialogContent>
    </Dialog>
  );
}

export function DiscardChangesDialog({ onCancel, onConfirm }: { onCancel: () => void; onConfirm: () => void }) {
  const { t } = useI18n();
  return (
    <Dialog open aria-labelledby="discard-title" onClose={onCancel}>
      <DialogTitle id="discard-title">{t("dialog.discard.title")}</DialogTitle>
      <DialogContent>
        <DialogContentText>{t("dialog.discard.body")}</DialogContentText>
      </DialogContent>
      <DialogActions>
        <Button type="button" onClick={onCancel}>{t("actions.keepEditing")}</Button>
        <Button aria-label={t("actions.discardChanges")} color="error" type="button" variant="contained" onClick={onConfirm}>{t("actions.discard")}</Button>
      </DialogActions>
    </Dialog>
  );
}
