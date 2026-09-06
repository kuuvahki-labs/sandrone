import { useMemo, useState } from "react";
import InfoOutlinedIcon from "@mui/icons-material/InfoOutlined";
import RefreshIcon from "@mui/icons-material/Refresh";
import SearchIcon from "@mui/icons-material/Search";
import Alert from "@mui/material/Alert";
import Button from "@mui/material/Button";
import Chip from "@mui/material/Chip";
import IconButton from "@mui/material/IconButton";
import InputAdornment from "@mui/material/InputAdornment";
import MenuItem from "@mui/material/MenuItem";
import Pagination from "@mui/material/Pagination";
import Paper from "@mui/material/Paper";
import Select from "@mui/material/Select";
import TextField from "@mui/material/TextField";
import Tooltip from "@mui/material/Tooltip";
import Typography from "@mui/material/Typography";

import { useLogs } from "~/features/settings/data/use-logs";
import { filterLogs, LOG_FILTERS, LOG_PAGE_SIZE, type LogFilter } from "~/features/settings/model/logs";
import type { ApiClient, LogEntry } from "~/shared/api/client";
import { useI18n } from "~/shared/i18n/context";
import { PageHeader } from "~/shared/ui/page";

const noEntries: LogEntry[] = [];
const levelColors = { debug: "default", info: "info", warn: "warning", error: "error" } as const;
const timestampOptions: Intl.DateTimeFormatOptions = {
  year: "numeric", month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", second: "2-digit",
  fractionalSecondDigits: 3, hour12: false,
};

export function SettingsLogsPage({ client, onBack }: { client: Pick<ApiClient, "getLogs">; onBack: () => void }) {
  const { t, locale } = useI18n();
  const timestampFormat = useMemo(() => new Intl.DateTimeFormat(locale, timestampOptions), [locale]);
  const { snapshot, loading, error, refresh, page, setPage } = useLogs(client);
  const [level, setLevel] = useState<LogFilter>("all");
  const [query, setQuery] = useState("");
  const entries = snapshot?.entries ?? noEntries;
  const filtered = useMemo(() => filterLogs(entries, level, query), [entries, level, query]);
  const updateFormat = useMemo(() => new Intl.DateTimeFormat(locale, { hour: "2-digit", minute: "2-digit", second: "2-digit", hour12: false }), [locale]);
  const visible = filtered.slice((page - 1) * LOG_PAGE_SIZE, page * LOG_PAGE_SIZE);

  return (
    <section className="grid min-w-0 gap-5">
      <PageHeader
        backAction={{ label: t("actions.back"), onSelect: onBack }}
        label=""
        title={t("settings.logs.title")}
        primaryAction={{
          label: t("settings.logs.refresh"), icon: <RefreshIcon aria-hidden fontSize="small" />,
          onSelect: () => void refresh(), disabled: loading, variant: "outlined",
        }}
        badge={snapshot ? <Typography color="text.secondary" variant="caption">{t("settings.logs.updated", { time: updateFormat.format(new Date(snapshot.snapshot_time)) })}</Typography> : undefined}
        sticky
      />
      {error ? <Alert severity="error" action={<Button disabled={loading} onClick={() => void refresh()}>{t("settings.logs.refresh")}</Button>}>{t("settings.logs.failed", { error })}</Alert> : null}
      {snapshot && snapshot.dropped > 0 ? <Alert severity="info">{t("settings.logs.dropped", { count: snapshot.dropped })}</Alert> : null}
      <div className="flex flex-wrap items-center gap-3">
        <Select
          inputProps={{ "aria-label": t("settings.logs.levels") }}
          size="small"
          value={level}
          onChange={(event) => { setLevel(event.target.value as LogFilter); setPage(1); }}
          className="min-w-36"
        >
          {LOG_FILTERS.map((filter) => <MenuItem key={filter} value={filter}>{t(`settings.logs.filter.${filter}`)}</MenuItem>)}
        </Select>
        <TextField
          className="order-3 min-w-0 basis-full sm:order-none sm:flex-1 sm:basis-40"
          placeholder={t("settings.logs.search")}
          slotProps={{
            htmlInput: { "aria-label": t("settings.logs.search") },
            input: { startAdornment: <InputAdornment position="start"><SearchIcon fontSize="small" /></InputAdornment> },
          }}
          size="small"
          value={query}
          onChange={(event) => { setQuery(event.target.value); setPage(1); }}
        />
        <div className="ml-auto flex shrink-0 items-center gap-1">
          <Typography aria-live="polite" color="text.secondary" variant="body2">
            {loading ? t("settings.logs.loading") : t(level === "all" && !query.trim() ? "settings.logs.count" : "settings.logs.filteredCount", { count: filtered.length, total: entries.length })}
          </Typography>
          <Tooltip describeChild title={[
            t("settings.logs.description"),
            snapshot ? t("settings.logs.level", { level: snapshot.level.toUpperCase() }) : "",
            snapshot ? t("settings.logs.updated", { time: timestampFormat.format(new Date(snapshot.snapshot_time)) }) : "",
          ].filter(Boolean).join(" · ")}>
            <IconButton size="small" aria-label={t("settings.logs.info")}><InfoOutlinedIcon fontSize="small" /></IconButton>
          </Tooltip>
        </div>
      </div>
      <Paper variant="outlined" className="min-w-0 overflow-hidden" aria-busy={loading}>
        {visible.map((entry) => (
          <details key={`${snapshot?.instance_id}:${entry.id}`} className="border-b border-divider last:border-b-0">
            <summary aria-label={t("settings.logs.details", { id: entry.id })} className="cursor-pointer px-4 py-3 focus-visible:outline-2 focus-visible:outline-primary-main">
              <span className="inline-grid w-[calc(100%-1.5rem)] min-w-0 gap-2 align-top sm:grid-cols-[auto_auto_minmax(0,1fr)] sm:items-center sm:gap-3">
                <time className="font-mono text-xs text-text-secondary" dateTime={entry.time}>{timestampFormat.format(new Date(entry.time))}</time>
                <span><Chip size="small" label={entry.level.toUpperCase()} color={levelColors[entry.level as keyof typeof levelColors] ?? "default"} /></span>
                <span className="min-w-0 truncate font-mono text-sm">{entry.message}</span>
              </span>
            </summary>
            <div className="grid min-w-0 gap-3 bg-action-hover px-4 pb-4 pt-2">
              {entry.truncated ? <Typography color="warning.main" variant="body2">{t("settings.logs.truncated")}</Typography> : null}
              <pre className="m-0 whitespace-pre-wrap break-words font-mono text-xs [overflow-wrap:anywhere]">{entry.message}</pre>
              <pre className="m-0 whitespace-pre-wrap break-words font-mono text-xs [overflow-wrap:anywhere]">{JSON.stringify(entry.attrs, null, 2)}</pre>
            </div>
          </details>
        ))}
        {!loading && snapshot && visible.length === 0 ? <Typography className="p-8 text-center" color="text.secondary">{t(entries.length === 0 ? "settings.logs.empty" : "settings.logs.noMatches")}</Typography> : null}
      </Paper>
      {filtered.length > LOG_PAGE_SIZE ? <Pagination aria-label={t("settings.logs.pagination")} count={Math.ceil(filtered.length / LOG_PAGE_SIZE)} page={page} onChange={(_, next) => setPage(next)} /> : null}
    </section>
  );
}
