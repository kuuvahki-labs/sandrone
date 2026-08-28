import { reactRouter } from "@react-router/dev/vite";
import tailwindcss from "@tailwindcss/vite";
import { defineConfig } from "vitest/config";

const backendTarget = process.env.SANDRONE_DEV_API_TARGET || "http://127.0.0.1:1137";
const muiOptimizerInclude = Object.freeze(["@mui/material/**"]);

export default defineConfig({
  plugins: [tailwindcss(), !process.env.VITEST && reactRouter()],
  build: {
    minify: "oxc",
    rolldownOptions: {
      output: {
        codeSplitting: {
          minSize: 20 * 1024,
          groups: [
            {
              name: "vendor",
              test: /node_modules[\\/]/,
              entriesAware: true,
              entriesAwareMergeThreshold: 32 * 1024,
              priority: 10,
            },
          ],
        },
      },
    },
  },
  resolve: {
    tsconfigPaths: true,
  },
  server: {
    proxy: {
      "/healthz": {
        target: backendTarget,
        changeOrigin: true,
      },
      "/version": {
        target: backendTarget,
        changeOrigin: true,
      },
      "/convert": {
        target: backendTarget,
        changeOrigin: true,
      },
      "/s/": {
        target: backendTarget,
        changeOrigin: true,
      },
      "/v1": {
        target: backendTarget,
        changeOrigin: true,
      },
    },
    allowedHosts:[
      '.local',
    ]
  },
  preview: {
    host: "127.0.0.1",
  },
  ssr: {
    noExternal: ["@mui/material", "@mui/icons-material"],
  },
  test: {
    exclude: ["**/node_modules/**", "**/e2e/**"],
    pool: "threads",
    isolate: true,
    maxWorkers: 4,
    testTimeout: 15_000,
    experimental: {
      fsModuleCache: true,
    },
    projects: [
      {
        extends: true,
        test: {
          name: "node",
          environment: "node",
          include: ["app/**/*.test.ts"],
          exclude: ["app/**/*.dom.test.ts"],
          deps: {
            optimizer: {
              ssr: {
                enabled: true,
                include: [...muiOptimizerInclude],
              },
            },
          },
        },
      },
      {
        extends: true,
        test: {
          name: "jsdom",
          environment: "jsdom",
          include: ["app/**/*.test.tsx", "app/**/*.dom.test.ts"],
          setupFiles: ["./app/test/setup.ts"],
          deps: {
            optimizer: {
              client: {
                enabled: true,
                include: [...muiOptimizerInclude],
              },
            },
          },
        },
      },
    ],
    server: {
      deps: {
        inline: ["@mui/material", "@mui/icons-material"],
      },
    },
  },
});
