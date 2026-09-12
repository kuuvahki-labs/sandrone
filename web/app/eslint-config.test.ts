import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

import { ESLint } from "eslint";
import { describe, expect, it } from "vitest";

const appDir = dirname(fileURLToPath(import.meta.url));
const webRoot = dirname(appDir);
const configFile = join(webRoot, "eslint.config.js");

function createESLint() {
  return new ESLint({ cwd: webRoot, overrideConfigFile: configFile });
}

describe("eslint frontend boundaries", () => {
  it("allows parent imports, re-exports, and MUI in routes", async () => {
    const eslint = createESLint();
    const [result] = await eslint.lintText(
      `import Button from "@mui/material/Button";

import { subscriptionSummary } from "../features/subscriptions/model/summary";

export { default } from "../features/files/pages/files-page";
export const Example = Button;
export const value = subscriptionSummary([]);
`,
      { filePath: join(appDir, "routes/example.tsx") },
    );

    expect(result.messages).toEqual([]);
  });

  it("rejects DOM globals only in node .test.ts files", async () => {
    const source = `void localStorage;
void sessionStorage;
void document;
void window;
void navigator;
`;
    const eslint = createESLint();
    const [[nodeResult], [domResult], [componentResult]] = await Promise.all([
      eslint.lintText(source, { filePath: join(appDir, "example.test.ts") }),
      eslint.lintText(source, { filePath: join(appDir, "example.dom.test.ts") }),
      eslint.lintText(source, { filePath: join(appDir, "example.test.tsx") }),
    ]);

    const nodeRestricted = nodeResult.messages
      .filter((message) => message.ruleId === "no-restricted-globals")
      .map((message) => message.message)
      .sort();
    expect(nodeRestricted).toHaveLength(5);
    expect(nodeRestricted.join("\n")).toContain("localStorage");
    expect(nodeRestricted.join("\n")).toContain("sessionStorage");
    expect(nodeRestricted.join("\n")).toContain("document");
    expect(nodeRestricted.join("\n")).toContain("window");
    expect(nodeRestricted.join("\n")).toContain("navigator");
    expect(domResult.messages.filter((message) => message.ruleId === "no-restricted-globals"))
      .toEqual([]);
    expect(componentResult.messages.filter((message) => message.ruleId === "no-restricted-globals"))
      .toEqual([]);
  });

  it("removes other browser-only globals from node tests while preserving Node globals", async () => {
    const browserOnlySource = `void HTMLElement;
void location;
void MutationObserver;
`;
    const nodeSource = `void URL;
void FormData;
void fetch;
void process;
void Buffer;
void structuredClone;
`;
    const eslint = createESLint();
    const [[nodeBrowserResult], [domResult], [componentResult], [nodeGlobalsResult]] = await Promise.all([
      eslint.lintText(browserOnlySource, { filePath: join(appDir, "example.test.ts") }),
      eslint.lintText(browserOnlySource, { filePath: join(appDir, "example.dom.test.ts") }),
      eslint.lintText(browserOnlySource, { filePath: join(appDir, "example.test.tsx") }),
      eslint.lintText(nodeSource, { filePath: join(appDir, "node-globals.test.ts") }),
    ]);

    const undefinedBrowserGlobals = nodeBrowserResult.messages
      .filter((message) => message.ruleId === "no-undef")
      .map((message) => message.message)
      .sort();
    expect(undefinedBrowserGlobals).toHaveLength(3);
    expect(undefinedBrowserGlobals.join("\n")).toContain("HTMLElement");
    expect(undefinedBrowserGlobals.join("\n")).toContain("location");
    expect(undefinedBrowserGlobals.join("\n")).toContain("MutationObserver");
    expect(domResult.messages.filter((message) => message.ruleId === "no-undef"))
      .toEqual([]);
    expect(componentResult.messages.filter((message) => message.ruleId === "no-undef"))
      .toEqual([]);
    expect(nodeGlobalsResult.messages.filter((message) => message.ruleId === "no-undef"))
      .toEqual([]);
  });
});
