import { describe, expect, it } from "vitest";

import viteConfig from "../vite.config";

interface ProxyOptions {
  target: string;
  changeOrigin: boolean;
}

interface OptimizerOptions {
  enabled?: boolean;
  include?: string[];
}

interface VitestProject {
  test?: {
    name?: string;
    deps?: {
      optimizer?: {
        client?: OptimizerOptions;
        ssr?: OptimizerOptions;
      };
    };
  };
}

function findVitestProject(name: string): VitestProject | undefined {
  const projects = (viteConfig as {
    test?: { projects?: VitestProject[] };
  }).test?.projects;

  return projects?.find((project) => project.test?.name === name);
}

describe("Vite development proxy", () => {
  it("proxies public backend endpoints to the health-check backend", () => {
    const proxy = (viteConfig as {
      server?: { proxy?: Record<string, ProxyOptions> };
    }).server?.proxy;

    expect(proxy?.["/healthz"]).toBeDefined();
    expect(proxy?.["/version"]).toEqual(proxy?.["/healthz"]);
    expect(proxy?.["/convert"]).toEqual(proxy?.["/healthz"]);
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

  it("persists transformed modules between warm test runs", () => {
    const experimental = (viteConfig as {
      test?: {
        experimental?: {
          fsModuleCache?: boolean;
        };
      };
    }).test?.experimental;

    expect(experimental?.fsModuleCache).toBe(true);
  });

  it("prebundles Material UI for Node tests through the SSR optimizer", () => {
    const optimizer = findVitestProject("node")?.test?.deps?.optimizer?.ssr;

    expect(optimizer?.enabled).toBe(true);
    expect(optimizer?.include).toEqual(["@mui/material/**"]);
  });

  it("prebundles Material UI for jsdom tests through the client optimizer", () => {
    const optimizer = findVitestProject("jsdom")?.test?.deps?.optimizer?.client;

    expect(optimizer?.enabled).toBe(true);
    expect(optimizer?.include).toEqual(["@mui/material/**"]);
  });
});
