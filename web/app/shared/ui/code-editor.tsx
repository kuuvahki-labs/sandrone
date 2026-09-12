import { type ChangeEventHandler, type CSSProperties, type ReactNode, useCallback, useId, useLayoutEffect, useRef, useState } from "react";
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

import { CodeExpansion } from "./code-expansion";
import { type CodeLanguage, JsonDiffCode, PrismCode } from "./code-highlight";
import { type CodeMatch, CodeSearchBar, findCodeMatches, useCodeSearch } from "./code-search";

export function CodeBlock({
  fillHeight = false,
  label,
  language = "text",
  showLanguage = true,
  toolbar,
  value,
}: {
  fillHeight?: boolean;
  label: string;
  language?: string;
  showLanguage?: boolean;
  toolbar?: ReactNode;
  value: string;
}) {
  const { t } = useI18n();
  const [copied, setCopied] = useState(false);

  async function copyCode() {
    try {
      await navigator.clipboard?.writeText(value);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1600);
    } catch {
      setCopied(false);
    }
  }

  return (
    <Paper
      aria-label={label}
      className={fillHeight ? "flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden" : "min-w-0 overflow-hidden"}
      component="section"
      role="region"
      variant="outlined"
    >
      <div className="flex min-w-0 flex-wrap items-center justify-between gap-3 border-b border-divider bg-background-paper px-3 py-2">
        <div className="flex min-w-0 flex-wrap items-center gap-2">
          <Typography className="break-words" component="h3" variant="subtitle2">
            {label}
          </Typography>
          {showLanguage ? (
            <Typography color="text.secondary" variant="caption">
              {language}
            </Typography>
          ) : null}
        </div>
        <div className="flex shrink-0 items-center gap-1">
          {toolbar}
          <Tooltip title={copied ? t("actions.copied") : t("actions.copy")}>
            <IconButton
              aria-label={`${copied ? t("actions.copied") : t("actions.copy")}${label}`}
              size="small"
              type="button"
              onClick={() => void copyCode()}
            >
              {copied ? <CheckIcon aria-hidden fontSize="small" /> : <ContentCopyIcon aria-hidden fontSize="small" />}
            </IconButton>
          </Tooltip>
        </div>
      </div>
      <pre
        className={
          fillHeight
            ? "m-0 min-h-0 flex-1 overflow-auto whitespace-pre bg-background-default p-3 text-xs text-text-primary"
            : "m-0 max-h-[min(70vh,640px)] overflow-auto whitespace-pre bg-background-default p-3 text-xs text-text-primary"
        }
      >
        {language === "json-diff" ? <JsonDiffCode value={value} /> : <PrismCode language={language} value={value} />}
      </pre>
    </Paper>
  );
}

export function HighlightedTextarea({
  className,
  defaultValue = "",
  editingTools,
  label,
  labelAction,
  language = "text",
  minRows = 4,
  name,
  onChange,
  placeholder,
  showLineNumbers = false,
  value,
}: {
  className?: string;
  defaultValue?: string;
  editingTools?: boolean;
  label: string;
  labelAction?: ReactNode;
  language?: CodeLanguage;
  minRows?: number;
  name?: string;
  onChange?: ChangeEventHandler<HTMLTextAreaElement>;
  placeholder?: string;
  showLineNumbers?: boolean;
  value?: string;
}) {
  const { t } = useI18n();
  const textareaId = useId();
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const searchButtonRef = useRef<HTMLButtonElement>(null);
  const highlightContentRef = useRef<HTMLDivElement>(null);
  const manuallySizedRef = useRef(false);
  const [uncontrolledValue, setUncontrolledValue] = useState(defaultValue);
  const [expanded, setExpanded] = useState(false);
  const isControlled = value !== undefined;
  // Use the textarea's newline normalization for both highlighting and offsets.
  const currentValue = (isControlled ? value : uncontrolledValue).replace(/\r\n?/g, "\n");
  const toolsEnabled = editingTools ?? showLineNumbers;
  const search = useCodeSearch(currentValue);
  const lineCount = Math.max(1, currentValue.split("\n").length);
  const rowCount = toolsEnabled ? Math.max(minRows, 8) : Math.max(minRows, 1);
  const minHeight = `${rowCount * 1.5 + 1.5}rem`;
  const editorStyle: CSSProperties = { minHeight: toolsEnabled ? `min(${minHeight}, 60dvh)` : minHeight };

  const syncHighlightScroll = useCallback((target: HTMLTextAreaElement) => {
    if (highlightContentRef.current) {
      highlightContentRef.current.style.transform = `translate(${-target.scrollLeft}px, ${-target.scrollTop}px)`;
    }
  }, []);

  const selectMatch = useCallback((match: CodeMatch) => {
    const textarea = textareaRef.current;
    if (!textarea) return;
    // Keep keyboard focus in the search field while showing the selected result.
    textarea.setSelectionRange(match.from, match.to);
    syncHighlightScroll(textarea);
    const mark = highlightContentRef.current?.querySelector<HTMLElement>(`[data-code-match-start="${match.from}"]`);
    if (mark) {
      const bounds = mark.getBoundingClientRect();
      const viewport = textarea.getBoundingClientRect();
      textarea.scrollTop += bounds.top - viewport.top - textarea.clientHeight / 3;
      textarea.scrollLeft += bounds.left - viewport.left - textarea.clientWidth / 3;
    }
    syncHighlightScroll(textarea);
  }, [syncHighlightScroll]);

  const updateValue: ChangeEventHandler<HTMLTextAreaElement> = (event) => {
    if (!isControlled) setUncontrolledValue(event.target.value);
    onChange?.(event);
  };

  useLayoutEffect(() => {
    const textarea = textareaRef.current;
    if (!textarea) return;
    const resize = () => {
      if (toolsEnabled && !expanded && !manuallySizedRef.current) {
        const style = getComputedStyle(textarea);
        const lineHeight = Number.parseFloat(style.lineHeight) || 24;
        const padding = (Number.parseFloat(style.paddingTop) || 12) + (Number.parseFloat(style.paddingBottom) || 12);
        textarea.style.height = `${Math.min(window.innerHeight * 0.6, Math.max(rowCount, lineCount) * lineHeight + padding)}px`;
      }
      syncHighlightScroll(textarea);
    };
    resize();
    window.addEventListener("resize", resize);
    return () => window.removeEventListener("resize", resize);
  }, [expanded, lineCount, rowCount, syncHighlightScroll, toolsEnabled]);

  useLayoutEffect(() => {
    if (textareaRef.current) syncHighlightScroll(textareaRef.current);
  }, [currentValue, syncHighlightScroll]);

  useLayoutEffect(() => {
    const textarea = textareaRef.current;
    if (!textarea || !search.query) return;
    const first = findCodeMatches(textarea.value, search.query)[0];
    if (first) selectMatch(first);
  }, [search.query, selectMatch]);

  function closeSearch() {
    search.close();
    searchButtonRef.current?.focus({ preventScroll: true });
  }

  return (
    <CodeExpansion
      className={className}
      expanded={expanded}
      label={label}
      onCancel={() => search.open ? closeSearch() : setExpanded(false)}
      onCollapse={() => setExpanded(false)}
      onKeyDown={(event) => {
        if (event.nativeEvent.isComposing) return;
        if (toolsEnabled && (event.ctrlKey || event.metaKey) && event.key.toLowerCase() === "f") {
          event.preventDefault();
          event.stopPropagation();
          search.show();
        } else if (event.key === "Escape" && (search.open || expanded)) {
          event.preventDefault();
          event.stopPropagation();
          if (search.open) closeSearch();
          else setExpanded(false);
        }
      }}
    >
      <div className="flex min-h-0 min-w-0 flex-1 flex-col gap-1.5" data-highlighted-textarea={language}>
        <div className="flex min-w-0 shrink-0 flex-wrap items-center justify-between gap-2" data-highlighted-textarea-label-row>
          <Typography className="min-w-0 flex-1 break-words" color="text.secondary" component="label" htmlFor={textareaId} variant="caption">
            {label}
          </Typography>
          {labelAction || toolsEnabled ? (
            <div className="flex shrink-0 items-center gap-1">
              {labelAction}
              {toolsEnabled ? (
                <>
                  <Tooltip title={t("code.search")}>
                    <IconButton aria-expanded={search.open} aria-label={t("code.searchIn", { label })} ref={searchButtonRef} size="small" type="button" onClick={() => search.open ? closeSearch() : search.show()}>
                      <SearchIcon aria-hidden fontSize="small" />
                    </IconButton>
                  </Tooltip>
                  <Tooltip title={t(expanded ? "code.collapseEdit" : "code.expandEdit")}>
                    <IconButton aria-label={t(expanded ? "code.collapseEdit" : "code.expandEdit")} aria-pressed={expanded} size="small" type="button" onClick={() => setExpanded((current) => !current)}>
                      {expanded ? <CloseFullscreenIcon aria-hidden fontSize="small" /> : <OpenInFullIcon aria-hidden fontSize="small" />}
                    </IconButton>
                  </Tooltip>
                </>
              ) : null}
            </div>
          ) : null}
        </div>
        <Paper className={`flex min-h-0 min-w-0 flex-col overflow-hidden border-divider bg-background-default focus-within:border-primary ${expanded ? "flex-1" : ""}`} variant="outlined">
          <CodeSearchBar label={label} search={search} onClose={() => searchButtonRef.current?.focus({ preventScroll: true })} onNavigate={selectMatch} />
          <div className={`relative min-h-0 min-w-0 ${expanded ? "flex-1" : ""}`}>
            <div
              aria-hidden
              className="pointer-events-none absolute inset-0 overflow-hidden whitespace-pre p-3 font-mono text-base leading-6 text-text-primary"
              data-highlighted-textarea-layer
            >
              <div className="min-w-max will-change-transform" data-highlighted-textarea-content ref={highlightContentRef}>
                {showLineNumbers ? (
                  <div className="grid min-w-0 grid-cols-[2.75rem_minmax(0,1fr)]">
                    <LineNumberGutter lineCount={lineCount} />
                    <div className="min-w-0">
                      <PrismCode activeMatch={search.activeMatch} language={language} matches={search.matches} value={currentValue || " "} />
                    </div>
                  </div>
                ) : (
                  <PrismCode activeMatch={search.activeMatch} language={language} matches={search.matches} value={currentValue || " "} />
                )}
              </div>
            </div>
            <textarea
              aria-label={label}
              className={[
                "highlighted-textarea-input relative block min-w-0 w-full resize-y overflow-auto whitespace-pre border-0 bg-transparent font-mono text-base leading-6 text-transparent caret-text-primary outline-none placeholder:text-text-secondary",
                showLineNumbers ? "py-3 pr-3 pl-14" : "p-3",
              ].join(" ")}
              id={textareaId}
              name={name}
              placeholder={placeholder}
              ref={textareaRef}
              spellCheck={false}
              style={editorStyle}
              value={currentValue}
              wrap="off"
              onChange={updateValue}
              onPointerDown={(event) => {
                const bounds = event.currentTarget.getBoundingClientRect();
                if (event.clientX >= bounds.right - 24 && event.clientY >= bounds.bottom - 24) manuallySizedRef.current = true;
              }}
              onScroll={(event) => syncHighlightScroll(event.currentTarget)}
            />
          </div>
        </Paper>
      </div>
    </CodeExpansion>
  );
}

function LineNumberGutter({ lineCount }: { lineCount: number }) {
  return (
    <div className="select-none pr-3 text-right font-mono text-text-secondary" data-highlighted-textarea-lines>
      {Array.from({ length: lineCount }, (_, index) => (
        <span className="block min-h-6 leading-6" data-line-number={index + 1} key={index + 1}>
          {index + 1}
        </span>
      ))}
    </div>
  );
}
