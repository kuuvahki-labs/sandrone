import ContentCopyIcon from "@mui/icons-material/ContentCopy";
import DeleteOutlinedIcon from "@mui/icons-material/DeleteOutlined";
import LinkIcon from "@mui/icons-material/Link";
import Chip from "@mui/material/Chip";
import List from "@mui/material/List";
import Typography from "@mui/material/Typography";

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
  onCopy: (item: ShareItem, format?: ShareCopyFormat) => void;
  onDelete: (item: ShareItem) => void;
}

export function SharesPage({ items, onCopy, onDelete }: SharesPageProps) {
  const { t } = useI18n();
  const validCount = items.filter((item) => item.status === "valid").length;
  const upcomingCount = items.filter((item) => item.status === "upcoming").length;
	const expiredCount = items.filter((item) => item.status === "expired").length;
	const exhaustedCount = items.filter((item) => item.status === "exhausted").length;
  return (
    <section className="grid gap-6">
      <PageHeader
        label=""
        title={t("shares.title")}
        metrics={(
          <div aria-label={t("shares.summary")} className="grid grid-cols-2 gap-4 md:grid-cols-4">
            <Metric label={t("shares.metric.valid")} value={validCount} />
            <Metric label={t("shares.metric.upcoming")} value={upcomingCount} />
            <Metric label={t("shares.metric.expired")} value={expiredCount} />
            <Metric label={t("shares.metric.exhausted")} value={exhaustedCount} />
          </div>
        )}
      />
      {items.length ? (
        <List aria-label={t("shares.list")} className="grid gap-3 p-0">
          {items.map((item) => (
            <DestinationListItem
              actions={shareActions(item, { onCopy, onDelete }, t)}
              icon={<LinkIcon aria-hidden color="action" />}
              key={item.id}
              meta={(
                <>
                  {shareStatusChip(item, t)}
				  {item.ageRecipient ? <Chip label="age X25519" size="small" variant="outlined" /> : null}
				  {item.maxUses ? <Chip label={t("shares.uses", { used: item.useCount ?? 0, max: item.maxUses })} size="small" variant="outlined" /> : null}
                  <Typography className="block break-words" component="code" variant="body2">
                    {item.publicUrl}
                  </Typography>
                </>
              )}
              primaryLabel={t("shares.actions.copy")}
              subtitle={item.targetKind === "subscription" && item.targetFormat ? `${item.targetName} · ${item.targetFormat}` : item.targetName}
              title={item.title}
              onPrimaryAction={() => onCopy(item)}
            />
          ))}
        </List>
      ) : (
        <EmptyState title={t("shares.empty")} />
      )}
    </section>
  );
}

function shareActions(
  item: ShareItem,
  handlers: {
    onCopy: (item: ShareItem, format?: ShareCopyFormat) => void;
    onDelete: (item: ShareItem) => void;
  },
  t: Translator,
): DestinationListAction[] {
  const copyActions: DestinationListAction[] = item.targetKind === "subscription"
    ? shareCopyFormats.map((entry) => ({
        icon: <ContentCopyIcon aria-hidden fontSize="small" />,
        label: t(entry.copyActionKey),
        onSelect: () => handlers.onCopy(item, entry.value),
      }))
    : [];
  return [
    ...copyActions,
    {
      icon: <DeleteOutlinedIcon aria-hidden fontSize="small" />,
      label: t("actions.delete"),
      onSelect: () => handlers.onDelete(item),
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
	case "exhausted":
	  return <Chip color="warning" label={t("shares.status.exhausted")} size="small" />;
  }
}
