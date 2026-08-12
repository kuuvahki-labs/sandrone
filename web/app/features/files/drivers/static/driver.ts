import type { FileDriverDefinition } from "~/features/files/drivers/core/file-driver";

export const staticFileDriverDefinition: FileDriverDefinition = {
  kind: "static",
  presentation: { labelKey: "files.kind.static", icon: "file" },
  configuration: { mode: "none" },
  createPresets: [
    { kind: "static", source: "local", sourceType: "inline", order: 0, initialName: "", icon: "file", labelKey: "model.fileSource.local", accessibleLabelKey: "files.create.local" },
    { kind: "static", source: "remote", sourceType: "remote", order: 1, initialName: "", icon: "remote", labelKey: "model.fileSource.remote", accessibleLabelKey: "files.create.remote" },
  ],
  source: {
    defaultBase: () => "",
    basePlaceholder: "",
    remoteURLPlaceholder: "https://example.com/file.txt",
    syntax: "text",
    strategy: "required",
    validate: () => null,
  },
  processors: {
    defaults: () => [],
    mergeModes: [],
    presets: [],
    validate: () => [],
  },
};
