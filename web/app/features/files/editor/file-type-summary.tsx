import Typography from "@mui/material/Typography";

import type { FileDriverIcon as FileDriverIconName } from "~/features/files/drivers/core/file-driver";

import { FileDriverIcon } from "./file-driver-icon";

export interface FileTypeSummaryProps {
  icon: FileDriverIconName;
  label: string;
  title: string;
}

export function FileTypeSummary({ icon, label, title }: FileTypeSummaryProps) {
  return (
    <div
      aria-label={title}
      className="flex min-w-0 items-center gap-3 rounded-md bg-action-hover p-3 md:col-span-2"
      role="group"
    >
      <div
        aria-hidden
        className="grid size-8 shrink-0 place-items-center rounded-md border border-divider bg-background-paper"
      >
        <FileDriverIcon icon={icon} />
      </div>
      <dl className="m-0 min-w-0">
        <Typography color="text.secondary" component="dt" variant="caption">
          {title}
        </Typography>
        <Typography className="m-0 break-words font-semibold" component="dd" variant="body2">
          {label}
        </Typography>
      </dl>
    </div>
  );
}
