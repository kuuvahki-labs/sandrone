import { useState } from "react";
import ContentCopyIcon from "@mui/icons-material/ContentCopy";
import DeleteOutlinedIcon from "@mui/icons-material/DeleteOutlined";
import LinkIcon from "@mui/icons-material/Link";
import Chip from "@mui/material/Chip";
import List from "@mui/material/List";
import Typography from "@mui/material/Typography";

import { ManualCopyDialog } from "~/features/shares/components/manual-copy-dialog";
import { ShareDetailsDialog } from "~/features/shares/components/share-details-dialog";
import { hasSelectionWithin, selectContents } from "~/features/shares/components/share-url-selection";
import type { CopyShareResult } from "~/features/shares/data/create-share-actions";
import { type ShareCopyFormat, shareCopyFormats } from "~/features/shares/model/share-formats";
import type { ShareItem } from "~/features/shares/model/types";
import { type Translator, useI18n } from "~/shared/i18n/context";
import { EmptyState } from "~/shared/ui/feedback";
import { Metric, PageHeader } from "~/shared/ui/page";
import {
  type DestinationListAction,
  DestinationListItem,
} from "~/shared/ui/resource-list";

export interface SharesPageProps {
  items: ShareItem[];
  loaded?: boolean;
  onCopy: (item: ShareItem, format?: ShareCopyFormat) => Promise<CopyShareResult>;
  onCopyUrl: (url: string) => Promise<CopyShareResult>;
  onDelete: (item: ShareItem) => void;
}

export function SharesPage({ items, loaded = true, onCopy, onCopyUrl, onDelete }: SharesPageProps) {
  const { t } = useI18n();
  const [manualCopyUrl, setManualCopyUrl] = useState<string | null>(null);
  const [selectedShare, setSelectedShare] = useState<ShareItem | null>(null);
  const validCount = items.filter((item) => item.status === "valid").length;
  const upcomingCount = items.filter((item) => item.status === "upcoming").length;
  const expiredCount = items.filter((item) => item.status === "expired").length;
  return (
    <section className="grid gap-6">
      <PageHeader
        label=""
        title={t("shares.title")}
        metrics={(
          <div aria-label={t("shares.summary")} className="grid grid-cols-2 gap-4 md:grid-cols-3">
            <Metric label={t("shares.metric.valid")} value={loaded ? validCount : undefined} />
            <Metric label={t("shares.metric.upcoming")} value={loaded ? upcomingCount : undefined} />
            <Metric label={t("shares.metric.expired")} value={loaded ? expiredCount : undefined} />
          </div>
        )}
      />
      {items.length ? (
        <List aria-label={t("shares.list")} className="grid gap-3 p-0">
          {items.map((item) => (
            <DestinationListItem
              actions={shareActions(item, {
                onCopy: async (format) => {
                  const copy = await onCopy(item, format);
                  if (!copy.copied) setManualCopyUrl(copy.url);
                },
                onDelete: () => onDelete(item),
              }, t)}
              icon={<LinkIcon aria-hidden color="action" />}
              key={item.id}
              meta={(
                <>
                  {shareStatusChip(item, t)}
                  {item.ageRecipient ? <Chip label="age X25519" size="small" variant="outlined" /> : null}
                  <Typography
                    className="block cursor-text break-words select-text"
                    component="code"
                    variant="body2"
                    onClick={(event) => {
                      event.stopPropagation();
                      if (!hasSelectionWithin(event.currentTarget)) {
                        selectContents(event.currentTarget);
                      }
                    }}
                  >
                    {item.publicUrl}
                  </Typography>
                </>
              )}
              primaryLabel={t("shares.actions.viewDetails")}
              subtitle={item.targetKind === "subscription" && item.targetFormat ? `${item.targetName} · ${item.targetFormat}` : item.targetName}
              title={item.title}
              onPrimaryAction={() => setSelectedShare(item)}
            />
          ))}
        </List>
      ) : loaded ? (
        <EmptyState title={t("shares.empty")} />
      ) : null}
      {selectedShare ? (
        <ShareDetailsDialog
          item={selectedShare}
          onClose={() => setSelectedShare(null)}
          onCopy={() => onCopy(selectedShare)}
        />
      ) : null}
      {manualCopyUrl ? (
        <ManualCopyDialog
          url={manualCopyUrl}
          onClose={() => setManualCopyUrl(null)}
          onRetry={() => onCopyUrl(manualCopyUrl)}
        />
      ) : null}
    </section>
  );
}

function shareActions(
  item: ShareItem,
  handlers: {
    onCopy: (format: ShareCopyFormat) => Promise<void>;
    onDelete: () => void;
  },
  t: Translator,
): DestinationListAction[] {
  const copyActions: DestinationListAction[] = item.targetKind === "subscription"
    ? shareCopyFormats.map((entry) => ({
        icon: <ContentCopyIcon aria-hidden fontSize="small" />,
        label: t(entry.copyActionKey),
        onSelect: () => { void handlers.onCopy(entry.value); },
      }))
    : [];
  return [
    ...copyActions,
    {
      icon: <DeleteOutlinedIcon aria-hidden fontSize="small" />,
      label: t("actions.delete"),
      onSelect: handlers.onDelete,
      tone: "danger",
    },
  ];
}

function shareStatusChip(item: ShareItem, t: Translator) {
  switch (item.status) {
    case "valid":
      return <Chip color="success" label={t("shares.status.valid")} size="small" />;
    case "upcoming":
      return <Chip color="info" label={t("shares.status.upcoming")} size="small" />;
    case "expired":
      return <Chip color="warning" label={t("shares.status.expired")} size="small" />;
  }
}
