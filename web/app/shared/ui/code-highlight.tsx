import { type ReactNode, useMemo } from "react";
import { Highlight, Prism, themes } from "prism-react-renderer";

import type { CodeMatch } from "./code-search";

export type CodeLanguage = "json" | "yaml" | "javascript" | "text" | "json-diff" | string;

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

export function JsonDiffCode({ value }: { value: string }) {
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

export function PrismCode({ language, value, showLineNumbers = false, wrap = false, matches = [], activeMatch }: {
  language: CodeLanguage;
  value: string;
  showLineNumbers?: boolean;
  wrap?: boolean;
  matches?: readonly CodeMatch[];
  activeMatch?: CodeMatch;
}) {
  const lineStarts = useMemo(() => [0, ...Array.from(value.matchAll(/\r\n|\r|\n/g), (match) => match.index + match[0].length)], [value]);

  return (
    <Highlight code={value} language={prismLanguage(language)} theme={themes.oneDark}>
      {({ getLineProps, getTokenProps, tokens }) => {
        let matchCursor = 0;
        return (
          <code className={`block max-w-none overflow-visible font-mono leading-6 language-${language}`}>
            {tokens.map((line, lineIndex) => {
              const lineProps = getLineProps({ line });
              let offset = lineStarts[lineIndex] ?? value.length;
              const content = line.map((token, tokenIndex) => {
                const start = offset;
                const end = start + token.content.length;
                offset = end;
                while (matchCursor < matches.length && matches[matchCursor].to <= start) matchCursor++;
                let cursor = start;
                const parts: ReactNode[] = [];
                for (let index = matchCursor; index < matches.length && matches[index].from < end; index++) {
                  const match = matches[index];
                  const from = Math.max(start, match.from);
                  const to = Math.min(end, match.to);
                  if (from > cursor) parts.push(token.content.slice(cursor - start, from - start));
                  parts.push(
                    <mark
                      className={activeMatch?.from === match.from ? "bg-warning/40 text-inherit outline outline-1 outline-warning" : "bg-warning/20 text-inherit"}
                      data-code-active-match={activeMatch?.from === match.from || undefined}
                      data-code-match-start={match.from}
                      key={match.from}
                    >
                      {token.content.slice(from - start, to - start)}
                    </mark>,
                  );
                  cursor = to;
                }
                if (cursor < end) parts.push(token.content.slice(cursor - start));
                return <span key={tokenIndex} {...getTokenProps({ token })}>{parts.length ? parts : token.content}</span>;
              });

              return (
                <span
                  key={lineIndex}
                  {...lineProps}
                  className={[
                    showLineNumbers ? "grid min-w-0 grid-cols-[2.75rem_minmax(0,1fr)]" : "block",
                    "min-h-6 leading-6",
                    lineProps.className,
                  ].filter(Boolean).join(" ")}
                  data-code-line={lineIndex + 1}
                >
                  {showLineNumbers ? <span aria-hidden className="select-none pr-3 text-right text-text-secondary">{lineIndex + 1}</span> : null}
                  <span className={wrap ? "min-w-0 whitespace-pre-wrap [overflow-wrap:anywhere]" : "whitespace-pre"} data-code-line-content>
                    {content}
                  </span>
                </span>
              );
            })}
          </code>
        );
      }}
    </Highlight>
  );
}
