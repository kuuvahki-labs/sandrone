import { type ReactNode, useId } from "react";
import Button from "@mui/material/Button";
import Dialog from "@mui/material/Dialog";
import DialogActions from "@mui/material/DialogActions";
import DialogContent from "@mui/material/DialogContent";
import DialogContentText from "@mui/material/DialogContentText";
import DialogTitle from "@mui/material/DialogTitle";

import { useI18n } from "~/shared/i18n/context";

export function AppDialog({
  children,
  disableClose = false,
  onClose,
  title,
}: {
  children: ReactNode;
  disableClose?: boolean;
  onClose: () => void;
  title: string;
}) {
  const titleId = useId();
  return (
    <Dialog
      fullWidth
      open
      aria-labelledby={titleId}
      maxWidth="sm"
      scroll="paper"
      onClose={() => {
        if (!disableClose) onClose();
      }}
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

export function OverwriteResourceDialog({
  name,
  onCancel,
  onConfirm,
  pending = false,
  resource,
}: {
  name: string;
  onCancel: () => void;
  onConfirm: () => void;
  pending?: boolean;
  resource: "file" | "subscription";
}) {
  const { t } = useI18n();
  const title = t(resource === "file" ? "dialog.overwrite.fileTitle" : "dialog.overwrite.subscriptionTitle");
  const body = t(resource === "file" ? "dialog.overwrite.fileBody" : "dialog.overwrite.subscriptionBody", { name });
  return (
    <Dialog
      open
      aria-labelledby="overwrite-title"
      onClose={() => {
        if (!pending) onCancel();
      }}
    >
      <DialogTitle id="overwrite-title">{title}</DialogTitle>
      <DialogContent>
        <DialogContentText>{body}</DialogContentText>
      </DialogContent>
      <DialogActions>
        <Button disabled={pending} type="button" onClick={onCancel}>{t("actions.cancel")}</Button>
        <Button color="error" disabled={pending} type="button" variant="contained" onClick={onConfirm}>{t("actions.overwrite")}</Button>
      </DialogActions>
    </Dialog>
  );
}
