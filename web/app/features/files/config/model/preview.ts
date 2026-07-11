import type { TranslationKey } from "~/shared/i18n/context";

import type {
  ConfigNodePreview,
  ConfigNodeSummary,
} from "./node-source";

export interface ConfigPreviewValidationInput {
  formMode: "create" | "edit";
  preview: ConfigNodePreview | null;
  projectedNodes: readonly ConfigNodeSummary[] | null;
  selected: boolean;
}

export interface ConfigPreviewValidation {
  issueKey?: TranslationKey;
  valid: boolean;
}

export interface ConfigPreviewStrategy {
  projectNodes: (preview: ConfigNodePreview) => ConfigNodeSummary[];
  relationNodeNames: (
    nodes: readonly ConfigNodeSummary[] | null,
    selected: boolean,
  ) => string[] | undefined;
  validate: (input: Readonly<ConfigPreviewValidationInput>) => ConfigPreviewValidation;
}
