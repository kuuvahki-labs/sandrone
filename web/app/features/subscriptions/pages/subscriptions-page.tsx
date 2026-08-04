import { useMemo, useState } from "react";
import AccountTreeOutlinedIcon from "@mui/icons-material/AccountTreeOutlined";
import CloudDownloadOutlinedIcon from "@mui/icons-material/CloudDownloadOutlined";
import DeleteOutlinedIcon from "@mui/icons-material/DeleteOutlined";
import LinkOutlinedIcon from "@mui/icons-material/LinkOutlined";
import OpenInNewIcon from "@mui/icons-material/OpenInNew";
import SearchIcon from "@mui/icons-material/Search";
import ShareOutlinedIcon from "@mui/icons-material/ShareOutlined";
import VisibilityOutlinedIcon from "@mui/icons-material/VisibilityOutlined";
import Chip from "@mui/material/Chip";
import InputAdornment from "@mui/material/InputAdornment";
import List from "@mui/material/List";
import TextField from "@mui/material/TextField";
import Typography from "@mui/material/Typography";

import { SubscriptionTrafficSummary } from "~/features/subscriptions/components/subscription-traffic-summary";
import { subscriptionSummary } from "~/features/subscriptions/model/summary";
import type { SubscriptionItem, SubscriptionKind, SubscriptionTraffic } from "~/features/subscriptions/model/types";
import { type Translator, useI18n } from "~/shared/i18n/context";
import { EmptyState } from "~/shared/ui/feedback";
import { Metric, PageHeader } from "~/shared/ui/page";
import {
  CreateSpeedDial,
  type CreateSpeedDialAction,
  type DestinationListAction,
  DestinationListItem,
} from "~/shared/ui/resource-list";

export interface SubscriptionsPageProps {
  createActions: CreateSpeedDialAction[];
  getTrafficKey?: (item: SubscriptionItem) => string;
  items: SubscriptionItem[];
  loaded?: boolean;
  trafficByKey?: Record<string, SubscriptionTraffic | null>;
  onDelete: (item: SubscriptionItem) => void;
  onEdit: (item: SubscriptionItem) => void;
  onPreview: (item: SubscriptionItem) => void;
  onShare: (item: SubscriptionItem) => void;
}

export function SubscriptionsPage({
  createActions,
  getTrafficKey = defaultTrafficKey,
  items,
  loaded = true,
  trafficByKey,
  onDelete,
  onEdit,
  onPreview,
  onShare,
}: SubscriptionsPageProps) {
  const { t } = useI18n();
  const [query, setQuery] = useState("");
  const [queryFocused, setQueryFocused] = useState(false);
  const summary = subscriptionSummary(items);
  const filtered = useMemo(() => {
    const normalized = query.trim().toLowerCase();
    if (!normalized) {
      return items;
    }
    return items.filter((item) => [item.title, item.name, item.format, item.description, item.warning].some((value) => String(value ?? "").toLowerCase().includes(normalized)));
  }, [items, query]);

  return (
    <section className="grid gap-6">
      <PageHeader
        label=""
        title={t("subscriptions.title")}
        metrics={(
          <div aria-label={t("subscriptions.summary")} className="grid grid-cols-2 gap-4 md:grid-cols-4">
            <Metric label={t("subscriptions.metric.total")} value={loaded ? summary.total : undefined} />
            <Metric label={t("subscriptions.metric.collection")} value={loaded ? summary.collections : undefined} />
            <Metric label={t("subscriptions.metric.remote")} value={loaded ? summary.remote : undefined} />
            <Metric label={t("subscriptions.metric.local")} value={loaded ? summary.local : undefined} />
          </div>
        )}
      />

      <div className="flex flex-col gap-3">
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
            htmlInput: { "aria-label": t("subscriptions.search") },
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

      {filtered.length ? (
        <List aria-label={t("subscriptions.list")} className="grid gap-3 p-0">
          {filtered.map((item) => {
            const trafficKey = getTrafficKey(item);
            const traffic = trafficByKey?.[trafficKey] ?? null;
            return (
              <DestinationListItem
                actions={subscriptionActions(item, traffic, { onDelete, onPreview, onShare }, t)}
                actionTitle={item.name}
                icon={subscriptionIcon(item.kind)}
                key={`${item.kind}:${item.name}`}
                details={traffic ? (
                  <SubscriptionListTraffic traffic={traffic} />
                ) : undefined}
                meta={(
                  <>
                    <Chip label={subscriptionKindLabel(item.kind, t)} size="small" variant="outlined" />
                    {item.warning ? (
                      <Typography className="break-words" color="warning.main" component="span" variant="body2">
                        {item.warning}
                      </Typography>
                    ) : null}
                  </>
                )}
                primaryLabel={t("actions.edit")}
                title={item.title}
                onPrimaryAction={() => onEdit(item)}
              />
            );
          })}
        </List>
      ) : loaded ? (
        <EmptyState title={t("subscriptions.empty")} />
      ) : null}
      <CreateSpeedDial actions={createActions} ariaLabel={t("subscriptions.create")} />
    </section>
  );
}

function SubscriptionListTraffic({
  traffic,
}: {
  traffic?: SubscriptionTraffic | null;
}) {
  if (!traffic) {
    return null;
  }
  return (
    <div className="grid min-w-0 gap-2">
      <SubscriptionTrafficSummary traffic={traffic} />
    </div>
  );
}

function defaultTrafficKey(item: SubscriptionItem): string {
  return `${item.kind}:${item.name}`;
}

function subscriptionActions(
  item: SubscriptionItem,
  traffic: SubscriptionTraffic | null,
  handlers: {
    onDelete: (item: SubscriptionItem) => void;
    onPreview: (item: SubscriptionItem) => void;
    onShare: (item: SubscriptionItem) => void;
  },
  t: Translator,
): DestinationListAction[] {
  const appUrl = subscriptionAppUrl(traffic);
  return [
    {
      accessibleLabel: t("subscriptions.actions.preview"),
      icon: <VisibilityOutlinedIcon aria-hidden fontSize="small" />,
      label: t("common.preview"),
      onSelect: () => handlers.onPreview(item),
    },
    ...(appUrl ? [{
      icon: <OpenInNewIcon aria-hidden fontSize="small" />,
      label: t("subscriptions.traffic.portal"),
      onSelect: () => window.open(appUrl, "_blank", "noopener,noreferrer"),
    }] satisfies DestinationListAction[] : []),
    {
      accessibleLabel: t("subscriptions.actions.share"),
      icon: <ShareOutlinedIcon aria-hidden fontSize="small" />,
      label: t("nav.shares"),
      onSelect: () => handlers.onShare(item),
    },
    {
      icon: <DeleteOutlinedIcon aria-hidden fontSize="small" />,
      label: t("actions.delete"),
      onSelect: () => handlers.onDelete(item),
      tone: "danger",
    },
  ];
}

function subscriptionAppUrl(traffic: SubscriptionTraffic | null): string {
  return traffic?.traffic?.appUrl ?? "";
}

function subscriptionIcon(kind: SubscriptionKind) {
  switch (kind) {
    case "remote":
      return <CloudDownloadOutlinedIcon aria-hidden color="action" />;
    case "local":
      return <LinkOutlinedIcon aria-hidden color="action" />;
    case "collection":
      return <AccountTreeOutlinedIcon aria-hidden color="primary" />;
  }
}

function subscriptionKindLabel(kind: SubscriptionKind, t: Translator): string {
  switch (kind) {
    case "remote":
      return t("model.subscription.remoteShort");
    case "local":
      return t("model.subscription.localShort");
    case "collection":
      return t("model.subscription.collectionShort");
  }
}
