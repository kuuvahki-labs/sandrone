import RefreshIcon from "@mui/icons-material/Refresh";
import ShareOutlinedIcon from "@mui/icons-material/ShareOutlined";
import Card from "@mui/material/Card";
import CardContent from "@mui/material/CardContent";
import Typography from "@mui/material/Typography";

import type { FilePreview } from "~/features/files/model/types";
import { useI18n } from "~/shared/i18n/context";
import { CollapsibleWarningPanel } from "~/shared/resources/warning-panel";
import { CodeBlock } from "~/shared/ui/code-editor";
import { PageHeader } from "~/shared/ui/page";

export interface FilePreviewPageProps {
  backLabel: string;
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
          ? "flex min-h-[calc(100dvh-2.5rem)] min-w-0 flex-col gap-4 min-[820px]:min-h-[calc(100dvh-3rem)]"
          : "grid min-w-0 gap-4"
      }
    >
      <PageHeader
        backAction={{ label: backLabel, onSelect: onBack }}
        label=""
        primaryAction={{ accessibleLabel: t("files.preview.refresh"), disabled: pending, icon: <RefreshIcon aria-hidden fontSize="small" />, label: t("actions.refresh"), onSelect: onRefresh }}
        secondaryActions={[{ accessibleLabel: t("files.actions.share"), icon: <ShareOutlinedIcon aria-hidden fontSize="small" />, label: t("actions.share"), onSelect: onShare }]}
        sticky
        title={t("files.preview.title")}
      />

      {pending && !preview ? (
        <Card component="article" variant="outlined">
          <CardContent>
            <div className="grid gap-2">
              <Typography component="h3" variant="h6">
                {t("files.preview.loadingTitle")}
              </Typography>
              <Typography color="text.secondary">{t("files.preview.loadingDescription")}</Typography>
            </div>
          </CardContent>
        </Card>
      ) : null}

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
            <CollapsibleWarningPanel label={t("files.preview.warnings")} warnings={preview.warnings} />
          ) : null}

          <div className="flex h-[min(70dvh,640px)] min-w-0 shrink-0">
            <CodeBlock
              fillHeight
              label={t("files.preview.finalContent")}
              language={languageFromPreview(preview.contentType, fileName)}
              value={preview.body}
            />
          </div>
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
