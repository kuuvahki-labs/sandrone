import { useMemo, useState } from "react";
import DeleteOutlinedIcon from "@mui/icons-material/DeleteOutlined";
import SearchIcon from "@mui/icons-material/Search";
import ShareOutlinedIcon from "@mui/icons-material/ShareOutlined";
import VisibilityOutlinedIcon from "@mui/icons-material/VisibilityOutlined";
import Chip from "@mui/material/Chip";
import InputAdornment from "@mui/material/InputAdornment";
import List from "@mui/material/List";
import TextField from "@mui/material/TextField";
import Typography from "@mui/material/Typography";

import { fileDriver } from "~/features/files/drivers/registry";
import { FileDriverIcon } from "~/features/files/editor/file-driver-icon";
import type { FileItem } from "~/features/files/model/types";
import { type Translator, useI18n } from "~/shared/i18n/context";
import { EmptyState } from "~/shared/ui/feedback";
import { Metric, PageHeader } from "~/shared/ui/page";
import {
  CreateSpeedDial,
  type CreateSpeedDialAction,
  type DestinationListAction,
  DestinationListItem,
} from "~/shared/ui/resource-list";

export interface FilesPageProps {
  createActions: CreateSpeedDialAction[];
  items: FileItem[];
  loaded?: boolean;
  onDelete: (item: FileItem) => void;
  onEdit: (item: FileItem) => void;
  onPreview: (item: FileItem) => void;
  onShare: (item: FileItem) => void;
}

export function FilesPage({ createActions, items, loaded = true, onDelete, onEdit, onPreview, onShare }: FilesPageProps) {
  const { t } = useI18n();
  const [query, setQuery] = useState("");
  const [queryFocused, setQueryFocused] = useState(false);
  const localCount = items.filter((item) => !isConfigFile(item) && item.sourceType !== "remote").length;
  const remoteCount = items.filter((item) => item.sourceType === "remote").length;
  const configCount = items.filter(isConfigFile).length;
  const filtered = useMemo(() => {
    const normalized = query.trim().toLowerCase();
    return items.filter((item) => {
      if (!normalized) return true;
      return [
        item.title,
        item.name,
        item.description,
        item.sourceSummary,
        item.kind,
        fileUsageLabel(item, t),
      ].some((value) => String(value ?? "").toLowerCase().includes(normalized));
    });
  }, [items, query, t]);

  return (
    <section className="grid gap-6">
      <PageHeader
        label=""
        metrics={(
          <div aria-label={t("files.summary")} className="grid grid-cols-2 gap-4 md:grid-cols-4">
            <Metric label={t("files.metric.files")} value={loaded ? items.length : undefined} />
            <Metric label={t("files.metric.local")} value={loaded ? localCount : undefined} />
            <Metric label={t("files.metric.remote")} value={loaded ? remoteCount : undefined} />
            <Metric label={t("files.metric.config")} value={loaded ? configCount : undefined} />
          </div>
        )}
        title={t("files.title")}
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
            htmlInput: { "aria-label": t("files.search") },
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
        <List aria-label={t("files.list")} className="grid gap-3 p-0">
          {filtered.map((item) => (
            <DestinationListItem
              actions={fileActions(item, { onDelete, onPreview, onShare }, t)}
              actionTitle={item.name}
              icon={fileIcon(item)}
              key={item.name}
              meta={(
                <>
                  <Chip label={fileUsageLabel(item, t)} size="small" variant="outlined" />
                </>
              )}
              primaryLabel={t("actions.edit")}
              subtitle={item.description ? <Typography color="text.secondary" component="span" variant="body2">{item.description}</Typography> : undefined}
              title={item.title}
              onPrimaryAction={() => onEdit(item)}
            />
          ))}
        </List>
      ) : loaded ? (
        <EmptyState title={t("files.empty")} />
      ) : null}
      <CreateSpeedDial actions={createActions} ariaLabel={t("files.create")} />
    </section>
  );
}

function fileIcon(item: FileItem) {
  const driver = fileDriver(item.kind);
  if (driver && driver.configuration.mode !== "none") return <FileDriverIcon icon={driver.presentation.icon} />;
  return <FileDriverIcon icon={isRemoteFile(item) ? "remote" : "file"} />;
}

function isRemoteFile(item: FileItem): boolean {
  return item.sourceType === "remote";
}

function fileUsageLabel(item: FileItem, t: Translator): string {
  const driver = fileDriver(item.kind);
  if (driver && driver.configuration.mode !== "none") return t(driver.presentation.labelKey);
  if (item.kind && !driver) return item.kind;
  if (item.sourceType === "remote") return t("files.usage.remote");
  return t("files.usage.local");
}

function isConfigFile(item: FileItem): boolean {
  const driver = fileDriver(item.kind);
  return driver !== undefined && driver.configuration.mode !== "none";
}

function fileActions(
  item: FileItem,
  handlers: {
    onDelete: (item: FileItem) => void;
    onPreview: (item: FileItem) => void;
    onShare: (item: FileItem) => void;
  },
  t: Translator,
): DestinationListAction[] {
  return [
    {
      accessibleLabel: t("files.actions.preview"),
      icon: <VisibilityOutlinedIcon aria-hidden fontSize="small" />,
      label: t("common.preview"),
      onSelect: () => handlers.onPreview(item),
    },
    {
      accessibleLabel: t("files.actions.share"),
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
