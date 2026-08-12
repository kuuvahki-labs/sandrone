import type { FileDriverDefinition } from "~/features/files/drivers/core/file-driver";
import { validateRemoteConfigSource } from "~/features/files/model/input-validation";

import { mihomoDefaultBase } from "./base";
import { mihomoConfigurationAdapter } from "./configuration";
import {
  defaultMihomoProcessors,
  mihomoProcessorPresets,
} from "./processor-presets";

export const mihomoFileDriverDefinition: FileDriverDefinition = {
  kind: "mihomo",
  presentation: { labelKey: "files.kind.mihomo", icon: "mihomo" },
  configuration: { mode: "structured", adapter: mihomoConfigurationAdapter },
  createPresets: [{ kind: "mihomo", source: "mihomo", sourceType: "inline", order: 2, initialName: "mihomo.yaml", icon: "mihomo", labelKey: "files.kind.mihomo", accessibleLabelKey: "files.create.mihomo" }],
  source: {
    defaultBase: mihomoDefaultBase,
    basePlaceholder: "mixed-port: 7890",
    remoteURLPlaceholder: "https://example.com/base.yaml",
    syntax: "yaml",
    strategy: "optional-base",
    validate: validateRemoteConfigSource,
  },
  processors: {
    defaults: defaultMihomoProcessors,
    mergeModes: ["yaml_overlay", "yaml_override"],
    presets: mihomoProcessorPresets,
    validate: () => [],
  },
};
