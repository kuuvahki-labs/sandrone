import { spawnSync } from "node:child_process";
import process from "node:process";

const pnpm = process.platform === "win32" ? "pnpm.cmd" : "pnpm";
const jsdomSmokeTests = [
  "app/test/integration/routing/app-routing-boot-auth.test.tsx",
  "app/shared/api/resource-list-cache.dom.test.ts",
  "app/shared/resources/use-resource-list.test.tsx",
  "app/shared/processors/components/processor-editor-list.test.tsx",
  "app/features/files/editor/file-form-driver.test.tsx",
  "app/features/settings/data/settings-data-hooks.test.tsx",
];

function run(command, args) {
  const result = spawnSync(command, args, { stdio: "inherit" });
  if (result.error) throw result.error;
  if (result.status !== 0) process.exit(result.status ?? 1);
}

run(pnpm, ["exec", "vitest", "--run", "--project", "node"]);
run(pnpm, ["exec", "vitest", "--run", "--project", "jsdom", ...jsdomSmokeTests]);
run(process.execPath, ["--test", "scripts/build-assets.test.mjs"]);
