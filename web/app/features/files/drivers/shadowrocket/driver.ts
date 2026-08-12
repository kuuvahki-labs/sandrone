import type { FileDriverDefinition } from "~/features/files/drivers/core/file-driver";
import { validateRemoteConfigSource } from "~/features/files/model/input-validation";

import { shadowrocketDefaultBase } from "./base";
import { shadowrocketConfigurationAdapter } from "./configuration";
import {
  defaultShadowrocketProcessors,
  shadowrocketProcessorPresets,
} from "./processor-presets";

export const shadowrocketFileDriverDefinition: FileDriverDefinition = {
  kind: "shadowrocket",
  targetRendererFormat: "shadowrocket-proxies",
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
    defaults: defaultShadowrocketProcessors,
    mergeModes: ["ini_override"],
    presets: shadowrocketProcessorPresets,
    validate: () => [],
  },
};
