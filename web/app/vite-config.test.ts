import { describe, expect, it } from "vitest";

import viteConfig from "../vite.config";

interface ProxyOptions {
  target: string;
  changeOrigin: boolean;
}

describe("Vite development proxy", () => {
  it("proxies the public version endpoint to the health-check backend", () => {
    const proxy = (viteConfig as {
      server?: { proxy?: Record<string, ProxyOptions> };
    }).server?.proxy;

    expect(proxy?.["/healthz"]).toBeDefined();
    expect(proxy?.["/version"]).toEqual(proxy?.["/healthz"]);
  });
});

describe("Vitest projects", () => {
  it("uses isolated threads with the verified worker count", () => {
    const test = (viteConfig as {
      test?: {
        pool?: string;
        isolate?: boolean;
        maxWorkers?: number;
      };
    }).test;

    expect(test?.pool).toBe("threads");
    expect(test?.isolate).toBe(true);
    expect(test?.maxWorkers).toBe(4);
  });

  it("routes browser-dependent TypeScript tests by a stable filename convention", () => {
    const projects = (viteConfig as {
      test?: {
        projects?: Array<{
          test?: {
            name?: string;
            include?: string[];
            exclude?: string[];
          };
        }>;
      };
    }).test?.projects;
    const nodeProject = projects?.find((project) => project.test?.name === "node");
    const jsdomProject = projects?.find((project) => project.test?.name === "jsdom");

    expect(nodeProject?.test?.exclude).toEqual(["app/**/*.dom.test.ts"]);
    expect(jsdomProject?.test?.include).toContain("app/**/*.dom.test.ts");
    expect(jsdomProject?.test?.include).toEqual([
      "app/**/*.test.tsx",
      "app/**/*.dom.test.ts",
    ]);
    expect(JSON.stringify(viteConfig)).not.toContain("app/lib/api.test.ts");
  });
});
