import type { FileDriverDefinition } from "~/features/files/drivers/core/file-driver";
import { validateRemoteConfigSource } from "~/features/files/model/input-validation";

import { shadowrocketDefaultBase } from "./base";
import { shadowrocketConfigurationAdapter } from "./configuration";

export const shadowrocketFileDriverDefinition: FileDriverDefinition = {
  kind: "shadowrocket",
  presentation: { labelKey: "files.kind.shadowrocket", icon: "rocket" },
  configuration: { mode: "structured", adapter: shadowrocketConfigurationAdapter },
  createPresets: [{ kind: "shadowrocket", source: "shadowrocket", sourceType: "inline", order: 4, initialName: "shadowrocket.conf", icon: "rocket", labelKey: "files.kind.shadowrocket", accessibleLabelKey: "files.create.shadowrocket" }],
  source: {
    defaultBase: shadowrocketDefaultBase,
    basePlaceholder: "[General]\nipv6 = false",
    remoteURLPlaceholder: "https://example.com/base.conf",
    syntax: "ini",
    strategy: "optional-base",
    validate: validateRemoteConfigSource,
  },
  processors: {
    defaults: () => [],
    mergeModes: ["ini_override"],
    presets: [],
    validate: () => [],
  },
};
