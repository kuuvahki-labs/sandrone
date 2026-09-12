import { useId, useState } from "react";
import CloseIcon from "@mui/icons-material/Close";
import Button from "@mui/material/Button";
import Drawer from "@mui/material/Drawer";
import IconButton from "@mui/material/IconButton";
import Typography from "@mui/material/Typography";

import { useI18n } from "~/shared/i18n/context";
import type { PreviewWarning } from "~/shared/resources/types";
import { groupPreviewWarnings } from "~/shared/resources/warning-groups";
import { WarningList } from "~/shared/resources/warnings";

export function FilePreviewWarnings({ warnings }: { warnings: readonly PreviewWarning[] }) {
  const { t } = useI18n();
  const [open, setOpen] = useState(false);
  const titleId = useId();
  const label = t("files.preview.warnings");

  return (
    <>
      <div aria-label={label} className="flex min-w-0 shrink-0 items-center justify-between gap-2" role="region">
        <Typography color="text.secondary" variant="body2">
          {t("warnings.summary", { groups: groupPreviewWarnings(warnings).length, warnings: warnings.length })}
        </Typography>
        <Button
          aria-expanded={open}
          aria-haspopup="dialog"
          aria-label={t("warnings.expandPanel", { label })}
          className="shrink-0"
          size="small"
          type="button"
          onClick={() => setOpen(true)}
        >
          {t("code.warningDetails")}
        </Button>
      </div>
      <Drawer
        anchor="right"
        open={open}
        slotProps={{ paper: { "aria-labelledby": titleId, role: "dialog", sx: { width: { xs: "100%", sm: 400 }, maxWidth: "100%" } } }}
        onClose={() => setOpen(false)}
      >
        <div className="flex shrink-0 items-center justify-between gap-3 border-b border-divider px-4 py-2">
          <Typography component="h3" id={titleId} variant="h6">{label}</Typography>
          <IconButton aria-label={t("actions.close")} type="button" onClick={() => setOpen(false)}>
            <CloseIcon aria-hidden />
          </IconButton>
        </div>
        <div className="min-h-0 min-w-0 flex-1 overflow-y-auto overscroll-contain p-4">
          <WarningList warnings={warnings} />
        </div>
      </Drawer>
    </>
  );
}
