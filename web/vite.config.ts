import { reactRouter } from "@react-router/dev/vite";
import tailwindcss from "@tailwindcss/vite";
import { defineConfig } from "vitest/config";

const backendTarget = process.env.SANDRONE_DEV_API_TARGET || "http://127.0.0.1:1137";

export default defineConfig({
  plugins: [tailwindcss(), !process.env.VITEST && reactRouter()],
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
    noExternal: ["@mui/material", "@mui/icons-material", "react-transition-group"],
  },
  test: {
    exclude: ["**/node_modules/**", "**/e2e/**"],
    maxWorkers: 4,
    testTimeout: 15_000,
    projects: [
      {
        extends: true,
        test: {
          name: "node",
          environment: "node",
          include: ["app/**/*.test.ts"],
          exclude: ["app/**/*.dom.test.ts"],
        },
      },
      {
        extends: true,
        test: {
          name: "jsdom",
          environment: "jsdom",
          include: ["app/**/*.test.tsx", "app/**/*.dom.test.ts"],
          setupFiles: ["./app/test/setup.ts"],
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
