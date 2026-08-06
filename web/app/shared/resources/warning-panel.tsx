import { useId, useState } from "react";
import KeyboardArrowDownIcon from "@mui/icons-material/KeyboardArrowDown";
import KeyboardArrowUpIcon from "@mui/icons-material/KeyboardArrowUp";
import ButtonBase from "@mui/material/ButtonBase";
import Card from "@mui/material/Card";
import Collapse from "@mui/material/Collapse";
import Typography from "@mui/material/Typography";

import { useI18n } from "~/shared/i18n/context";
import type { PreviewWarning } from "~/shared/resources/types";
import { groupPreviewWarnings } from "~/shared/resources/warning-groups";
import { WarningList } from "~/shared/resources/warnings";

export function CollapsibleWarningPanel({ label, warnings }: { label: string; warnings: readonly PreviewWarning[] }) {
  const { t } = useI18n();
  const [expanded, setExpanded] = useState(false);
  const detailsId = useId();
  const summary = t("warnings.summary", { groups: groupPreviewWarnings(warnings).length, warnings: warnings.length });

  return (
    <Card aria-label={label} className="min-w-0 overflow-hidden" component="section" variant="outlined">
      <ButtonBase
        aria-controls={detailsId}
        aria-expanded={expanded}
        aria-label={t(expanded ? "warnings.collapsePanel" : "warnings.expandPanel", { label })}
        className="block w-full p-4 text-left"
        type="button"
        onClick={() => setExpanded((value) => !value)}
      >
        <span className="grid min-w-0 w-full grid-cols-[minmax(0,1fr)_auto] items-center gap-3">
          <span className="grid min-w-0 gap-1">
            <Typography component="h3" variant="h6">
              {t("common.warning")}
            </Typography>
            <Typography color="text.secondary" component="span" variant="body2">
              {summary}
            </Typography>
          </span>
          <span className="text-text-secondary">
            {expanded ? <KeyboardArrowUpIcon aria-hidden fontSize="small" /> : <KeyboardArrowDownIcon aria-hidden fontSize="small" />}
          </span>
        </span>
      </ButtonBase>
      <Collapse id={detailsId} in={expanded} timeout="auto" unmountOnExit>
        <div className="min-w-0 border-t border-divider p-4">
          <WarningList showSummary={false} warnings={warnings} />
        </div>
      </Collapse>
    </Card>
  );
}
