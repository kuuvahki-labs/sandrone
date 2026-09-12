import RefreshIcon from "@mui/icons-material/Refresh";
import ShareOutlinedIcon from "@mui/icons-material/ShareOutlined";
import Card from "@mui/material/Card";
import CardContent from "@mui/material/CardContent";
import Typography from "@mui/material/Typography";

import type { FilePreview } from "~/features/files/model/types";
import { useI18n } from "~/shared/i18n/context";
import { PreviewPendingStatus } from "~/shared/preview/preview-pending-status";
import { FileContentPreview } from "~/shared/ui/file-content-preview";
import { PageHeader } from "~/shared/ui/page";

import { FilePreviewWarnings } from "./file-preview-warnings";

export interface FilePreviewPageProps {
  backLabel: string;
  elapsedSeconds?: number;
  failed?: boolean;
  fileName: string;
  pending?: boolean;
  preview?: FilePreview;
  onBack: () => void;
  onRefresh: () => void;
  onShare: () => void;
}

export function FilePreviewPage({
  backLabel,
  elapsedSeconds = 0,
  failed = false,
  fileName,
  pending = false,
  preview,
  onBack,
  onRefresh,
  onShare,
}: FilePreviewPageProps) {
  const { t } = useI18n();

  return (
    <section
      className={
        preview
          ? "flex h-[calc(100dvh-2.5rem)] min-h-0 min-w-0 flex-col gap-3 min-[820px]:h-[calc(100dvh-3rem)]"
          : "grid min-w-0 gap-4"
      }
    >
      <div className="shrink-0">
        <PageHeader
          backAction={{ label: backLabel, onSelect: onBack }}
          label=""
          primaryAction={{ accessibleLabel: t("files.preview.refresh"), disabled: pending, icon: <RefreshIcon aria-hidden fontSize="small" />, label: t("actions.refresh"), onSelect: onRefresh }}
          secondaryActions={[{ accessibleLabel: t("files.actions.share"), icon: <ShareOutlinedIcon aria-hidden fontSize="small" />, label: t("actions.share"), onSelect: onShare }]}
          sticky
          title={t("files.preview.title")}
        />
      </div>

      {pending && !preview ? <PreviewPendingStatus elapsedSeconds={elapsedSeconds} /> : null}

      {failed && !preview ? (
        <Card component="article" variant="outlined">
          <CardContent>
            <div className="grid gap-2">
              <Typography component="h3" variant="h6">
                {t("files.preview.errorTitle")}
              </Typography>
              <Typography color="text.secondary">{t("files.preview.errorDescription")}</Typography>
            </div>
          </CardContent>
        </Card>
      ) : null}

      {preview ? (
        <>
          {preview.warnings.length ? (
            <FilePreviewWarnings warnings={preview.warnings} />
          ) : null}

          <FileContentPreview
            label={t("files.preview.finalContent")}
            language={languageFromPreview(preview.contentType, fileName)}
            value={preview.body}
          />
        </>
      ) : null}
    </section>
  );
}

function languageFromPreview(contentType: string, fileName: string): string {
  const normalizedContentType = contentType.toLowerCase();
  if (normalizedContentType.includes("json")) {
    return "json";
  }
  if (normalizedContentType.includes("yaml") || normalizedContentType.includes("yml")) {
    return "yaml";
  }
  if (normalizedContentType.includes("javascript") || fileName.toLowerCase().endsWith(".js")) {
    return "javascript";
  }
  if (fileName.toLowerCase().endsWith(".conf")) {
    return "ini";
  }
  return "text";
}
