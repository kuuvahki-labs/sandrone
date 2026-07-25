import { useEffect, useRef } from "react";
import Button from "@mui/material/Button";
import DialogActions from "@mui/material/DialogActions";
import Typography from "@mui/material/Typography";

import type { CopyShareResult } from "~/features/shares/data/create-share-actions";
import { useI18n } from "~/shared/i18n/context";
import { AppDialog } from "~/shared/ui/dialogs";

import { hasSelectionWithin, selectContents } from "./share-url-selection";

export function ManualCopyDialog({
  onClose,
  onRetry,
  url,
}: {
  onClose: () => void;
  onRetry: () => Promise<CopyShareResult>;
  url: string;
}) {
  const { t } = useI18n();
  const urlElement = useRef<HTMLElement | null>(null);

  useEffect(() => {
    const timeout = window.setTimeout(() => {
      selectContents(urlElement.current);
    });
    return () => window.clearTimeout(timeout);
  }, [url]);

  async function retryCopy() {
    const copy = await onRetry();
    if (copy.copied) {
      onClose();
    } else {
      selectContents(urlElement.current);
    }
  }

  return (
    <AppDialog title={t("shares.manualCopy.title")} onClose={onClose}>
      <Typography
        className="block cursor-text break-words select-text"
        component="code"
        ref={urlElement}
        onClick={(event) => {
          if (!hasSelectionWithin(event.currentTarget)) {
            selectContents(event.currentTarget);
          }
        }}
      >
        {url}
      </Typography>
      <DialogActions className="px-0 pb-0">
        <Button type="button" onClick={onClose}>{t("share.result.done")}</Button>
        <Button type="button" variant="contained" onClick={() => { void retryCopy(); }}>
          {t("shares.manualCopy.retry")}
        </Button>
      </DialogActions>
    </AppDialog>
  );
}
