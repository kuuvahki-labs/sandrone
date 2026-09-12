import { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";
import CheckIcon from "@mui/icons-material/Check";
import CloseFullscreenIcon from "@mui/icons-material/CloseFullscreen";
import ContentCopyIcon from "@mui/icons-material/ContentCopy";
import OpenInFullIcon from "@mui/icons-material/OpenInFull";
import SearchIcon from "@mui/icons-material/Search";
import IconButton from "@mui/material/IconButton";
import Paper from "@mui/material/Paper";
import Tooltip from "@mui/material/Tooltip";
import Typography from "@mui/material/Typography";

import { useI18n } from "~/shared/i18n/context";
import { CodeExpansion } from "~/shared/ui/code-expansion";
import { PrismCode } from "~/shared/ui/code-highlight";
import { type CodeMatch, CodeSearchBar, useCodeSearch } from "~/shared/ui/code-search";

export function FileContentPreview({ label, language, value }: { label: string; language: string; value: string }) {
  const { t } = useI18n();
  const [expanded, setExpanded] = useState(false);
  const [copyState, setCopyState] = useState<"idle" | "copied" | "failed">("idle");
  const copyTimer = useRef<number | undefined>(undefined);
  const scrollRef = useRef<HTMLPreElement>(null);
  const previousQuery = useRef("");
  const search = useCodeSearch(value);
  const copyLabel = copyState === "copied" ? t("actions.copied") : copyState === "failed" ? t("code.copyFailed") : t("actions.copy");
  const expandLabel = t(expanded ? "code.collapsePreview" : "code.expandPreview");

  useEffect(() => () => window.clearTimeout(copyTimer.current), []);

  const revealMatch = useCallback((match: CodeMatch | undefined) => {
    if (!match) return;
    const scroller = scrollRef.current;
    const mark = scroller?.querySelector<HTMLElement>(`[data-code-match-start="${match.from}"]`);
    if (!scroller || !mark) return;
    const viewport = scroller.getBoundingClientRect();
    const bounds = mark.getBoundingClientRect();
    if (bounds.top < viewport.top || bounds.bottom > viewport.bottom) {
      scroller.scrollTop += bounds.top - viewport.top - scroller.clientHeight / 3;
    }
  }, []);

  useLayoutEffect(() => {
    if (previousQuery.current === search.query) return;
    previousQuery.current = search.query;
    revealMatch(search.activeMatch);
  }, [revealMatch, search.activeMatch, search.query]);

  async function copyContent() {
    window.clearTimeout(copyTimer.current);
    try {
      if (!navigator.clipboard?.writeText) throw new Error("Clipboard unavailable");
      await navigator.clipboard.writeText(value);
      setCopyState("copied");
    } catch {
      setCopyState("failed");
    }
    copyTimer.current = window.setTimeout(() => setCopyState("idle"), 1600);
  }

  function closeSearch() {
    search.close();
    scrollRef.current?.focus({ preventScroll: true });
  }

  return (
    <CodeExpansion
      className="flex min-h-0 min-w-0 flex-1"
      expanded={expanded}
      label={label}
      onCancel={() => { if (search.open) closeSearch(); else setExpanded(false); }}
      onCollapse={() => setExpanded(false)}
    >
      <Paper
        aria-label={label}
        className="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden"
        component="section"
        role="region"
        variant="outlined"
        onKeyDown={(event) => {
          if (event.nativeEvent.isComposing) return;
          if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === "f") {
            event.preventDefault();
            event.stopPropagation();
            search.show();
          } else if (event.key === "Escape" && search.open) {
            event.preventDefault();
            event.stopPropagation();
            closeSearch();
          }
        }}
      >
        <div className="flex min-w-0 shrink-0 items-center justify-between gap-2 border-b border-divider bg-background-paper px-3 py-2">
          <div className="flex min-w-0 items-baseline gap-2">
            <Typography className="truncate" component="h3" variant="subtitle2">{label}</Typography>
            <Typography className="hidden sm:block" color="text.secondary" variant="caption">{language}</Typography>
          </div>
          <div className="flex shrink-0 items-center gap-1">
            <Tooltip title={t("code.search")}>
              <IconButton aria-label={t("code.searchIn", { label })} size="small" type="button" onClick={search.show}>
                <SearchIcon aria-hidden fontSize="small" />
              </IconButton>
            </Tooltip>
            <Tooltip title={copyLabel}>
              <IconButton aria-label={`${copyLabel}${label}`} size="small" type="button" onClick={() => void copyContent()}>
                {copyState === "copied" ? <CheckIcon aria-hidden fontSize="small" /> : <ContentCopyIcon aria-hidden fontSize="small" />}
              </IconButton>
            </Tooltip>
            <Tooltip title={expandLabel}>
              <IconButton aria-label={expandLabel} size="small" type="button" onClick={() => setExpanded((current) => !current)}>
                {expanded ? <CloseFullscreenIcon aria-hidden fontSize="small" /> : <OpenInFullIcon aria-hidden fontSize="small" />}
              </IconButton>
            </Tooltip>
          </div>
          <span className="sr-only" role="status">{copyState === "idle" ? "" : copyLabel}</span>
        </div>
        {search.open ? <CodeSearchBar label={label} search={search} onClose={() => scrollRef.current?.focus({ preventScroll: true })} onNavigate={revealMatch} /> : null}
        {/* Keyboard users need to scroll the content and open its local find bar. */}
        {/* eslint-disable-next-line jsx-a11y/no-noninteractive-tabindex */}
        <pre tabIndex={0}
          aria-label={label}
          className="m-0 min-h-0 min-w-0 flex-1 overflow-auto overscroll-contain whitespace-pre-wrap bg-background-default p-3 text-sm text-text-primary outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-primary"
          ref={scrollRef}
        >
          <PrismCode activeMatch={search.activeMatch} language={language} matches={search.open ? search.matches : undefined} showLineNumbers value={value} wrap />
        </pre>
      </Paper>
    </CodeExpansion>
  );
}
