import type { FileDriverDefinition } from "./core/file-driver";
import { createFileDriverRegistry, type FileDriverRegistry } from "./core/registry";
import { mihomoFileDriverDefinition } from "./mihomo/driver";
import { shadowrocketFileDriverDefinition } from "./shadowrocket/driver";
import { singBoxFileDriverDefinition } from "./sing-box/driver";
import { staticFileDriverDefinition } from "./static/driver";

export const FILE_DRIVER_REGISTRY: FileDriverRegistry = createFileDriverRegistry([
  staticFileDriverDefinition,
  mihomoFileDriverDefinition,
  singBoxFileDriverDefinition,
  shadowrocketFileDriverDefinition,
]);

export function fileDriver(
  kind: string | null | undefined,
): Readonly<FileDriverDefinition> | undefined {
  return FILE_DRIVER_REGISTRY.get(kind);
}

export function requireFileDriver(kind: string): Readonly<FileDriverDefinition> {
  const driver = fileDriver(kind);
  if (!driver) throw new Error(`unregistered file kind: ${kind || "(missing)"}`);
  return driver;
}
