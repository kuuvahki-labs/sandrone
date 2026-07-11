import type { ComponentType } from "react";

import type {
  GroupFieldsDraft,
  RuleDraft,
  RuleSetDraft,
} from "~/features/files/config/model/editor-model";

export interface ConfigFieldSlotProps<Draft> {
  draft: Readonly<Draft>;
  onUpdate: (patch: Partial<Draft>) => void;
}

export interface GroupFieldsProps extends ConfigFieldSlotProps<GroupFieldsDraft> {
  healthCheck: boolean;
  index: number;
}

export interface RuleSetFieldsProps extends ConfigFieldSlotProps<RuleSetDraft> {
  behaviorOptions: readonly { value: string; label: string }[];
}

export interface RuleFieldsProps extends ConfigFieldSlotProps<RuleDraft> {
  supportsNoResolve: boolean;
}

export type RuleSetHeaderLayout =
  | "name"
  | "name-fields"
  | "name-fields-source"
  | "name-source";

export type RuleSetSummaryField = "behavior" | "format";

export interface RuleSetPresentation {
  headerLayout: RuleSetHeaderLayout;
  intervalInputType: "number" | "text";
  remoteFields: "format-interval" | "url-only";
  sourceMode: "remote-only" | "switchable";
  summaryFields: readonly RuleSetSummaryField[];
}

export interface StructuredConfigurationFieldSlots {
  GroupFields: ComponentType<GroupFieldsProps>;
  RuleFields: ComponentType<RuleFieldsProps>;
  RuleSetFields: ComponentType<RuleSetFieldsProps>;
  ruleSetPresentation: Readonly<RuleSetPresentation>;
}
