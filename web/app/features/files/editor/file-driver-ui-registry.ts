import { mihomoConfigurationFields } from "~/features/files/drivers/mihomo/fields";
import { shadowrocketConfigurationFields } from "~/features/files/drivers/shadowrocket/fields";
import { singBoxConfigurationFields } from "~/features/files/drivers/sing-box/fields";

import type { StructuredConfigurationFieldSlots } from "./file-driver-ui";

export type FileDriverUIRegistry = Readonly<
  Record<string, Readonly<StructuredConfigurationFieldSlots>>
>;

export const FILE_DRIVER_UI_REGISTRY: FileDriverUIRegistry = Object.freeze({
  mihomo: freezeStructuredConfigurationUI(mihomoConfigurationFields),
  "sing-box": freezeStructuredConfigurationUI(singBoxConfigurationFields),
  shadowrocket: freezeStructuredConfigurationUI(shadowrocketConfigurationFields),
});

export function fileDriverUI(
  kind: string | null | undefined,
): Readonly<StructuredConfigurationFieldSlots> | undefined {
  return kind && Object.hasOwn(FILE_DRIVER_UI_REGISTRY, kind)
    ? FILE_DRIVER_UI_REGISTRY[kind]
    : undefined;
}

export function requireFileDriverUI(
  kind: string,
): Readonly<StructuredConfigurationFieldSlots> {
  const ui = fileDriverUI(kind);
  if (!ui) throw new Error(`unregistered file driver UI: ${kind || "(missing)"}`);
  return ui;
}

function freezeStructuredConfigurationUI(
  ui: StructuredConfigurationFieldSlots,
): Readonly<StructuredConfigurationFieldSlots> {
  return Object.freeze({
    ...ui,
    ruleSetPresentation: Object.freeze({
      ...ui.ruleSetPresentation,
      summaryFields: Object.freeze([...ui.ruleSetPresentation.summaryFields]),
    }),
  });
}
