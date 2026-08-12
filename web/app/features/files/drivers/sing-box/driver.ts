import type { FileDriverDefinition } from "~/features/files/drivers/core/file-driver";
import {
  validateJSONConfigSource,
  validateJSONMergeProcessors,
} from "~/features/files/model/input-validation";

import { singBoxDefaultBase } from "./base";
import { singBoxConfigurationAdapter } from "./configuration";
import {
  defaultSingBoxProcessors,
  singBoxProcessorPresets,
} from "./processor-presets";

export const singBoxFileDriverDefinition: FileDriverDefinition = {
  kind: "sing-box",
  presentation: { labelKey: "files.kind.singBox", icon: "sing-box" },
  configuration: { mode: "structured", adapter: singBoxConfigurationAdapter, requiresValidOnCreate: true },
  createPresets: [{ kind: "sing-box", source: "sing-box", sourceType: "inline", order: 3, initialName: "sing-box.json", icon: "sing-box", labelKey: "files.kind.singBox", accessibleLabelKey: "files.create.singBox" }],
  source: {
    defaultBase: singBoxDefaultBase,
    basePlaceholder: "{\n  \"log\": { \"level\": \"info\" }\n}",
    remoteURLPlaceholder: "https://example.com/base.json",
    syntax: "json",
    strategy: "optional-base",
    validate: validateJSONConfigSource,
  },
  processors: {
    defaults: defaultSingBoxProcessors,
    mergeModes: ["json_overlay", "json_override"],
    presets: singBoxProcessorPresets,
    validate: validateJSONMergeProcessors,
  },
};
