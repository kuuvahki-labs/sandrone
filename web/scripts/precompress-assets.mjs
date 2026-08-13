import { promisify } from "node:util";
import process from "node:process";
import {
  constants,
  brotliCompress as brotliCompressCallback,
} from "node:zlib";
import { readdir, readFile, writeFile } from "node:fs/promises";
import { extname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const brotliCompress = promisify(brotliCompressCallback);
const compressibleExtensions = new Set([".css", ".js"]);

export const brotliThresholdBytes = 8 * 1024;

export async function precompressAssets(root) {
  const outputs = [];
  for (const file of await walkFiles(resolve(root))) {
    if (!compressibleExtensions.has(extname(file))) continue;
    const body = await readFile(file);
    if (body.length < brotliThresholdBytes) continue;
    const compressed = await brotliCompress(body, {
      params: {
        [constants.BROTLI_PARAM_MODE]: constants.BROTLI_MODE_TEXT,
        [constants.BROTLI_PARAM_QUALITY]: 11,
        [constants.BROTLI_PARAM_SIZE_HINT]: body.length,
      },
    });
    const output = `${file}.br`;
    await writeFile(output, compressed);
    outputs.push(output);
  }
  return outputs;
}

async function walkFiles(root) {
  const files = [];
  for (const entry of await readdir(root, { withFileTypes: true })) {
    const file = resolve(root, entry.name);
    if (entry.isDirectory()) {
      files.push(...await walkFiles(file));
    } else if (entry.isFile()) {
      files.push(file);
    }
  }
  return files;
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const root = resolve(process.argv[2] ?? "build/client");
  const outputs = await precompressAssets(root);
  process.stdout.write(`Generated ${outputs.length} Brotli assets.\n`);
}
