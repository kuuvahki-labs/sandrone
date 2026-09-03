import { useMemo, useState } from "react";
import ContentCopyIcon from "@mui/icons-material/ContentCopy";
import DeleteOutlinedIcon from "@mui/icons-material/DeleteOutlined";
import LinkIcon from "@mui/icons-material/Link";
import SearchIcon from "@mui/icons-material/Search";
import TransformOutlinedIcon from "@mui/icons-material/TransformOutlined";
import Chip from "@mui/material/Chip";
import InputAdornment from "@mui/material/InputAdornment";
import List from "@mui/material/List";
import TextField from "@mui/material/TextField";
import Typography from "@mui/material/Typography";

import { ManualCopyDialog } from "~/features/shares/components/manual-copy-dialog";
import { ShareDetailsDialog } from "~/features/shares/components/share-details-dialog";
import { hasSelectionWithin, selectContents } from "~/features/shares/components/share-url-selection";
import type { CopyShareResult } from "~/features/shares/data/create-share-actions";
import { type ShareCopyFormat, shareCopyFormats } from "~/features/shares/model/share-formats";
import type { ShareItem } from "~/features/shares/model/types";
import { type Translator, useI18n } from "~/shared/i18n/context";
import { EmptyState } from "~/shared/ui/feedback";
import { Metric, MetricGroup, PageHeader } from "~/shared/ui/page";
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
  onGenerateConvertLink: () => void;
}

export function SharesPage({ items, loaded = true, onCopy, onCopyUrl, onDelete, onGenerateConvertLink }: SharesPageProps) {
  const { t } = useI18n();
  const [manualCopyUrl, setManualCopyUrl] = useState<string | null>(null);
  const [query, setQuery] = useState("");
  const [queryFocused, setQueryFocused] = useState(false);
  const [selectedShare, setSelectedShare] = useState<ShareItem | null>(null);
  const [statusFilter, setStatusFilter] = useState<ShareStatusFilter>("all");
  const [targetFilter, setTargetFilter] = useState<ShareTargetFilter>("all");
  const validCount = items.filter((item) => item.status === "valid").length;
  const unavailableCount = items.length - validCount;
  const subscriptionCount = items.filter((item) => item.targetKind === "subscription").length;
  const fileCount = items.filter((item) => item.targetKind === "file").length;
  const filteredItems = useMemo(() => {
    const normalized = query.trim().toLowerCase();
    return items.filter((item) => {
      if (!shareMatchesStatusFilter(item, statusFilter)) return false;
      if (targetFilter !== "all" && item.targetKind !== targetFilter) return false;
      if (!normalized) return true;
      return [
        item.title,
        item.id,
        item.targetName,
        item.targetFormat,
        item.publicUrl,
        shareStatusLabel(item.status, t),
        item.targetKind ? t(`shares.details.targetKind.${item.targetKind}`) : "",
      ].some((value) => String(value ?? "").toLowerCase().includes(normalized));
    });
  }, [items, query, statusFilter, t, targetFilter]);
  return (
    <section className="grid gap-6">
      <PageHeader
        label=""
        secondaryActions={[{
          accessibleLabel: t("shares.convert.action"),
          icon: <TransformOutlinedIcon aria-hidden fontSize="small" />,
          label: t("shares.convert.action"),
          mobileIconOnly: true,
          mobileVisible: true,
          onSelect: onGenerateConvertLink,
        }]}
        title={t("shares.title")}
        metrics={(
          <MetricGroup label={t("shares.summary")}>
            <Metric
              actionLabel={t("actions.filterBy", { label: t("shares.metric.valid") })}
              label={t("shares.metric.valid")}
              selected={statusFilter === "valid"}
              value={loaded ? validCount : undefined}
              onSelect={() => setStatusFilter((current) => current === "valid" ? "all" : "valid")}
            />
            <Metric
              actionLabel={t("actions.filterBy", { label: t("shares.metric.unavailable") })}
              label={t("shares.metric.unavailable")}
              selected={statusFilter === "unavailable"}
              value={loaded ? unavailableCount : undefined}
              onSelect={() => setStatusFilter((current) => current === "unavailable" ? "all" : "unavailable")}
            />
            <Metric
              actionLabel={t("actions.filterBy", { label: t("nav.subscriptions") })}
              label={t("nav.subscriptions")}
              selected={targetFilter === "subscription"}
              value={loaded ? subscriptionCount : undefined}
              onSelect={() => setTargetFilter((current) => current === "subscription" ? "all" : "subscription")}
            />
            <Metric
              actionLabel={t("actions.filterBy", { label: t("nav.files") })}
              label={t("nav.files")}
              selected={targetFilter === "file"}
              value={loaded ? fileCount : undefined}
              onSelect={() => setTargetFilter((current) => current === "file" ? "all" : "file")}
            />
          </MetricGroup>
        )}
      />
      <div className="grid gap-3">
        <TextField
          fullWidth
          label={t("actions.search")}
          type="search"
          value={query}
          slotProps={{
            input: {
              startAdornment: (
                <InputAdornment position="start">
                  <SearchIcon aria-hidden fontSize="small" />
                </InputAdornment>
              ),
            },
            htmlInput: { "aria-label": t("shares.search") },
            inputLabel: {
              shrink: queryFocused || Boolean(query),
              sx: {
                "&:not(.MuiInputLabel-shrink)": {
                  transform: "translate(48px, 16px) scale(1)",
                },
              },
            },
          }}
          onBlur={() => setQueryFocused(false)}
          onChange={(event) => setQuery(event.target.value)}
          onFocus={() => setQueryFocused(true)}
        />
      </div>
      {filteredItems.length ? (
        <List aria-label={t("shares.list")} className="grid gap-3 p-0">
          {filteredItems.map((item) => (
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
                    className="block min-w-0 w-full basis-full cursor-text truncate select-text"
                    component="code"
                    title={item.publicUrl}
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
        <EmptyState title={items.length ? t("filters.empty") : t("shares.empty")} />
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

type ShareStatusFilter = "all" | "unavailable" | "valid";
type ShareTargetFilter = "all" | NonNullable<ShareItem["targetKind"]>;

function shareMatchesStatusFilter(item: ShareItem, filter: ShareStatusFilter): boolean {
  if (filter === "all") return true;
  if (filter === "valid") return item.status === "valid";
  return item.status === "upcoming" || item.status === "expired";
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
  return <Chip color={shareStatusColor(item.status)} label={shareStatusLabel(item.status, t)} size="small" />;
}

function shareStatusLabel(status: ShareItem["status"], t: Translator): string {
  return t(`shares.status.${status}`);
}

function shareStatusColor(status: ShareItem["status"]): "info" | "success" | "warning" {
  switch (status) {
    case "valid":
      return "success";
    case "upcoming":
      return "info";
    case "expired":
      return "warning";
  }
}
