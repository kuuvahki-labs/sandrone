import { type ChangeEventHandler, type CSSProperties, type ReactNode, useCallback, useId, useLayoutEffect, useRef, useState } from "react";
import CheckIcon from "@mui/icons-material/Check";
import ContentCopyIcon from "@mui/icons-material/ContentCopy";
import IconButton from "@mui/material/IconButton";
import Paper from "@mui/material/Paper";
import Tooltip from "@mui/material/Tooltip";
import Typography from "@mui/material/Typography";
import { Highlight, Prism, themes } from "prism-react-renderer";

import { useI18n } from "~/shared/i18n/context";

type CodeLanguage = "json" | "yaml" | "javascript" | "text" | "json-diff" | string;

if (!Prism.languages.ini) {
  Prism.languages.ini = {
    comment: /^[ \t]*[;#].*$/m,
    selector: /^[ \t]*\[.*?\]/m,
    constant: /^[ \t]*[^\s=]+?(?=[ \t]*=)/m,
    "attr-value": {
      pattern: /=.*/,
      inside: {
        punctuation: /^=/,
      },
    },
  };
}

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
  const textareaId = useId();
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const highlightContentRef = useRef<HTMLDivElement>(null);
  const [uncontrolledValue, setUncontrolledValue] = useState(defaultValue);
  const isControlled = value !== undefined;
  const currentValue = isControlled ? value : uncontrolledValue;
  const lineCount = Math.max(1, currentValue.split("\n").length);
  const minHeight = `${Math.max(minRows, 1) * 1.5 + 1.5}rem`;
  const editorStyle: CSSProperties = { minHeight };

  const syncHighlightScroll = useCallback((target: HTMLTextAreaElement) => {
    if (highlightContentRef.current) {
      highlightContentRef.current.style.transform = `translate(${-target.scrollLeft}px, ${-target.scrollTop}px)`;
    }
  }, []);

  const updateValue: ChangeEventHandler<HTMLTextAreaElement> = (event) => {
    if (!isControlled) {
      setUncontrolledValue(event.target.value);
    }
    onChange?.(event);
  };

  useLayoutEffect(() => {
    if (textareaRef.current) {
      syncHighlightScroll(textareaRef.current);
    }
  }, [currentValue, syncHighlightScroll]);

  return (
    <div className={["grid gap-1.5", className].filter(Boolean).join(" ")} data-highlighted-textarea={language}>
      <div className="flex min-w-0 items-center justify-between gap-2" data-highlighted-textarea-label-row>
        <Typography className="min-w-0 break-words" color="text.secondary" component="label" htmlFor={textareaId} variant="caption">
          {label}
        </Typography>
        {labelAction ? <div className="flex shrink-0 items-center gap-1">{labelAction}</div> : null}
      </div>
      <Paper className="relative min-w-0 overflow-hidden border-divider bg-background-default focus-within:border-primary" variant="outlined">
        <div
          aria-hidden
          className="pointer-events-none absolute bottom-0 left-0 right-0 top-0 overflow-hidden whitespace-pre p-3 font-mono text-base leading-6 text-text-primary"
          data-highlighted-textarea-layer
          style={editorStyle}
        >
          <div className="min-w-max will-change-transform" data-highlighted-textarea-content ref={highlightContentRef}>
            {showLineNumbers ? (
              <div className="grid min-w-0 grid-cols-[2.75rem_minmax(0,1fr)]">
                <LineNumberGutter lineCount={lineCount} />
                <div className="min-w-0">
                  <PrismCode language={language} value={currentValue || " "} />
                </div>
              </div>
            ) : (
              <PrismCode language={language} value={currentValue || " "} />
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
          onScroll={(event) => {
            syncHighlightScroll(event.currentTarget);
          }}
        />
      </Paper>
    </div>
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

function PrismCode({ language, value }: { language: CodeLanguage; value: string }) {
  return (
    <Highlight code={value} language={prismLanguage(language)} theme={themes.oneDark}>
      {({ getLineProps, getTokenProps, tokens }) => (
        <code className={`block max-w-none overflow-visible font-mono leading-6 language-${language}`}>
          {tokens.map((line, lineIndex) => {
            const lineProps = getLineProps({ line });
            return (
              <span
                key={lineIndex}
                {...lineProps}
                className={["block min-h-6 leading-6", lineProps.className].filter(Boolean).join(" ")}
              >
                {line.map((token, tokenIndex) => {
                  const tokenProps = getTokenProps({ token });
                  return <span key={tokenIndex} {...tokenProps} />;
                })}
              </span>
            );
          })}
        </code>
      )}
    </Highlight>
  );
}

function JsonDiffCode({ value }: { value: string }) {
  const lines = value.split("\n");
  return (
    <code className="block language-json-diff">
      {lines.map((line, index) => {
        const state = diffLineState(line);
        const prefix = state === "unchanged" ? "" : line.slice(0, 1);
        const jsonText = state === "unchanged" ? line : line.slice(1);
        return (
          <span
            className={["block min-h-5 min-w-full w-max code-diff-line", state === "added" ? "code-diff-line-added bg-success/10" : "", state === "removed" ? "code-diff-line-removed bg-error/10" : ""].filter(Boolean).join(" ")}
            data-diff-line={state}
            key={`${state}-${index}`}
          >
            {prefix ? <span className="inline-block w-4 select-none font-semibold">{prefix}</span> : <span className="inline-block w-4 select-none" />}
            <PrismInlineCode language="json" value={jsonText} />
          </span>
        );
      })}
    </code>
  );
}

function PrismInlineCode({ language, value }: { language: CodeLanguage; value: string }) {
  return (
    <Highlight code={value || " "} language={prismLanguage(language)} theme={themes.oneDark}>
      {({ getTokenProps, tokens }) => (
        <>
          {(tokens[0] ?? []).map((token, tokenIndex) => {
            const tokenProps = getTokenProps({ token });
            return <span key={tokenIndex} {...tokenProps} />;
          })}
        </>
      )}
    </Highlight>
  );
}

function diffLineState(line: string): "added" | "removed" | "unchanged" {
  if (line.startsWith("+")) return "added";
  if (line.startsWith("-")) return "removed";
  return "unchanged";
}

function prismLanguage(language: CodeLanguage): string {
  if (language === "json-diff") return "json";
  if (language === "js") return "javascript";
  if (language === "yml") return "yaml";
  if (language === "txt") return "text";
  return language || "text";
}
