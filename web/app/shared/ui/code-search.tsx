import { useLayoutEffect, useMemo, useRef, useState } from "react";
import ArrowDownwardIcon from "@mui/icons-material/ArrowDownward";
import ArrowUpwardIcon from "@mui/icons-material/ArrowUpward";
import CloseIcon from "@mui/icons-material/Close";
import IconButton from "@mui/material/IconButton";
import TextField from "@mui/material/TextField";
import Tooltip from "@mui/material/Tooltip";
import Typography from "@mui/material/Typography";

import { useI18n } from "~/shared/i18n/context";

export interface CodeMatch {
  from: number;
  to: number;
}

export function findCodeMatches(value: string, query: string): CodeMatch[] {
  if (!query) return [];
  const expression = new RegExp(query.replace(/[.*+?^${}()|[\]\\]/g, "\\$&"), "giu");
  return Array.from(value.matchAll(expression), (match) => ({ from: match.index, to: match.index + match[0].length }));
}

export function useCodeSearch(value: string) {
  const [open, setOpen] = useState(false);
  const [focusRequest, requestFocus] = useState(0);
  const [query, updateQuery] = useState("");
  const [index, setIndex] = useState(0);
  const matches = useMemo(() => findCodeMatches(value, query), [query, value]);
  const activeIndex = matches.length ? Math.min(index, matches.length - 1) : 0;

  return {
    open,
    focusRequest,
    query,
    matches,
    activeIndex,
    activeMatch: matches[activeIndex],
    setQuery(next: string) {
      updateQuery(next);
      setIndex(0);
    },
    show() {
      setOpen(true);
      requestFocus((current) => current + 1);
    },
    close() {
      setOpen(false);
      updateQuery("");
      setIndex(0);
    },
    move(direction: 1 | -1) {
      if (!matches.length) return undefined;
      const next = (activeIndex + direction + matches.length) % matches.length;
      setIndex(next);
      return matches[next];
    },
  };
}

export function CodeSearchBar({ label, search, onNavigate, onClose }: {
  label: string;
  search: ReturnType<typeof useCodeSearch>;
  onNavigate?: (match: CodeMatch) => void;
  onClose?: () => void;
}) {
  const { t } = useI18n();
  const inputRef = useRef<HTMLInputElement>(null);
  useLayoutEffect(() => {
    if (search.open) {
      inputRef.current?.focus();
      inputRef.current?.select();
    }
  }, [search.focusRequest, search.open]);

  function navigate(direction: 1 | -1) {
    const match = search.move(direction);
    if (match) onNavigate?.(match);
  }

  function close() {
    search.close();
    onClose?.();
  }

  if (!search.open) return null;

  return (
    <div
      aria-label={t("code.searchIn", { label })}
      className="flex min-w-0 shrink-0 items-center gap-1 border-b border-divider bg-background-paper p-2"
      role="search"
    >
      <TextField
        className="min-w-0 flex-1"
        inputRef={inputRef}
        placeholder={t("code.search")}
        size="small"
        slotProps={{ htmlInput: { "aria-label": t("code.searchIn", { label }) } }}
        type="search"
        value={search.query}
        onKeyDown={(event) => {
          if (event.nativeEvent.isComposing) return;
          if (event.key === "Enter") {
            event.preventDefault();
            event.stopPropagation();
            navigate(event.shiftKey ? -1 : 1);
          } else if (event.key === "Escape") {
            event.preventDefault();
            event.stopPropagation();
            close();
          }
        }}
        onChange={(event) => {
          // Search is presentation state, not a change to the enclosing resource form.
          event.stopPropagation();
          search.setQuery(event.target.value);
        }}
      />
      <Typography aria-live="polite" className="shrink-0 whitespace-nowrap px-1" color="text.secondary" role="status" variant="caption">
        {t("code.matchCount", { current: search.matches.length ? search.activeIndex + 1 : 0, total: search.matches.length })}
      </Typography>
      <Tooltip title={t("code.previousMatch")}>
        <span><IconButton aria-label={t("code.previousMatch")} disabled={!search.matches.length} size="small" type="button" onClick={() => navigate(-1)}><ArrowUpwardIcon fontSize="small" /></IconButton></span>
      </Tooltip>
      <Tooltip title={t("code.nextMatch")}>
        <span><IconButton aria-label={t("code.nextMatch")} disabled={!search.matches.length} size="small" type="button" onClick={() => navigate(1)}><ArrowDownwardIcon fontSize="small" /></IconButton></span>
      </Tooltip>
      <Tooltip title={t("code.closeSearch")}>
        <IconButton aria-label={t("code.closeSearch")} size="small" type="button" onClick={close}><CloseIcon fontSize="small" /></IconButton>
      </Tooltip>
    </div>
  );
}
