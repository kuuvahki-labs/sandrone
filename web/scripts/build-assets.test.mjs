import assert from "node:assert/strict";
import { Buffer } from "node:buffer";
import { brotliDecompressSync } from "node:zlib";
import { mkdtemp, readFile, rm, stat, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import { URL } from "node:url";

import viteConfig from "../vite.config.ts";
import {
  brotliThresholdBytes,
  precompressAssets,
} from "./precompress-assets.mjs";

test("production build explicitly minifies and coalesces small shared chunks", () => {
  assert.equal(viteConfig.build?.minify, "oxc");
  const codeSplitting = viteConfig.build?.rolldownOptions?.output?.codeSplitting;
  assert.equal(codeSplitting.minSize, 20 * 1024);
  assert.deepEqual(codeSplitting.groups.map((group) => group.name), ["vendor"]);
  assert.equal(codeSplitting.groups[0]?.entriesAwareMergeThreshold, 32 * 1024);
});

test("production build precompresses the generated client assets", async () => {
  const packageJSON = JSON.parse(await readFile(new URL("../package.json", import.meta.url), "utf8"));
  assert.match(packageJSON.scripts.build, /node scripts\/precompress-assets\.mjs build\/client$/u);
});

test("precompressAssets writes Brotli variants only for eligible JS and CSS", async (t) => {
  const root = await mkdtemp(join(tmpdir(), "sandrone-web-assets-"));
  t.after(async () => {
    await rm(root, { force: true, recursive: true });
  });

  const largeJS = Buffer.from("const sandrone = true;\n".repeat(512));
  const largeCSS = Buffer.from(".sandrone{display:block}\n".repeat(384));
  const smallJS = Buffer.alloc(brotliThresholdBytes - 1, 0x61);
  const image = Buffer.alloc(brotliThresholdBytes + 1, 0x62);
  await writeFile(join(root, "app.js"), largeJS);
  await writeFile(join(root, "app.css"), largeCSS);
  await writeFile(join(root, "small.js"), smallJS);
  await writeFile(join(root, "logo.png"), image);

  await precompressAssets(root);

  assert.deepEqual(brotliDecompressSync(await readFile(join(root, "app.js.br"))), largeJS);
  assert.deepEqual(brotliDecompressSync(await readFile(join(root, "app.css.br"))), largeCSS);
  await assert.rejects(stat(join(root, "small.js.br")), { code: "ENOENT" });
  await assert.rejects(stat(join(root, "logo.png.br")), { code: "ENOENT" });
});
