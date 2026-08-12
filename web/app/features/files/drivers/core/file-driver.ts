import type {
  AdaptiveGroupAnchorProblem,
  AdaptiveGroupGeneration,
  AdaptiveGroupMergeResult,
  AdaptiveGroupOptions,
  AdaptiveGroupStaleInput,
  AdaptiveGroupStripResult,
} from "~/features/files/config/model/adaptive-groups";
import type {
  AddCatalogRuleSetResult,
  ConfigEditorDraft,
  ConfigMap,
  GroupDraft,
  ProxyGroupMemberMode,
  RuleDraft,
  RuleSetDraft,
} from "~/features/files/config/model/editor-model";
import type { ConfigNamingLocale } from "~/features/files/config/model/naming";
import type { ConfigPreviewStrategy } from "~/features/files/config/model/preview";
import type { ConfigReferenceStrategy } from "~/features/files/config/model/references";
import type { ConfigRelationStrategy, ConfigValidationIssue } from "~/features/files/config/model/relations";
import type { ConfigTemplateStrategy } from "~/features/files/config/model/templates";
import type {
  FileInputValidationCode,
  FileProcessorValidationIssue,
} from "~/features/files/model/input-validation";
import type {
  FileAdaptiveGroupConfigDetail,
  FileConfigDetail,
  FileConfigDraft,
  FileSourceDetail,
  RuleSetCatalogItem,
  RuleSetCatalogTarget,
} from "~/features/files/model/types";
import type { Translator } from "~/shared/i18n/context";
import type { ProcessorDraft } from "~/shared/processors/model";
import type { ProcessorDetail } from "~/shared/resources/types";

export type FileMergeMode = "yaml_overlay" | "yaml_override" | "json_overlay" | "json_override" | "ini_override";
export type FileDriverIcon = string;

export interface FileCreatePreset {
  kind: string;
  source: string;
  sourceType: "inline" | "remote";
  order: number;
  initialName: string;
  icon?: FileDriverIcon;
  labelKey?: Parameters<Translator>[0];
  accessibleLabelKey?: Parameters<Translator>[0];
}

export interface ConfigAdaptiveStrategy {
  anchorProblem: (config: Readonly<FileConfigDraft>) => AdaptiveGroupAnchorProblem | null;
  canonicalNames: (groups: readonly ConfigMap[]) => string[];
  configFromOptions: (options: Readonly<AdaptiveGroupOptions>) => FileAdaptiveGroupConfigDetail | undefined;
  defaultOptions: () => AdaptiveGroupOptions;
  generate: (
    nodeNames: readonly string[],
    options: Readonly<AdaptiveGroupOptions>,
    namingLocale?: ConfigNamingLocale,
  ) => AdaptiveGroupGeneration;
  initiallyEnabled: (
    formMode: "create" | "edit",
    config: FileAdaptiveGroupConfigDetail | undefined,
  ) => boolean;
  isStale: (input: Readonly<AdaptiveGroupStaleInput>) => boolean;
  merge: (
    config: Readonly<FileConfigDraft>,
    generation: Readonly<AdaptiveGroupGeneration>,
  ) => AdaptiveGroupMergeResult;
  optionsFromConfig: (config: FileAdaptiveGroupConfigDetail | undefined) => AdaptiveGroupOptions;
  recognizesCanonicalLayer: (config: Readonly<FileConfigDraft>) => boolean;
  requiresNodePreview: boolean;
  strip: (config: Readonly<FileConfigDraft>) => AdaptiveGroupStripResult;
  typeOptions: readonly { label: string; value: string }[];
}

export interface StructuredGroupDraftAdapter {
  create: (namingLocale?: ConfigNamingLocale) => GroupDraft;
  defaults: (preset: string, namingLocale?: ConfigNamingLocale) => GroupDraft[];
  isHealthCheck: (type: string) => boolean;
  project: (groups: ConfigMap[]) => GroupDraft[] | null;
  serialize: (groups: GroupDraft[]) => ConfigMap[];
  supportsExcludeFilter: boolean;
  supportsHidden: boolean;
  supportsRuntimeFilter: boolean;
  transitionMemberMode: (group: GroupDraft, mode: ProxyGroupMemberMode, restoredMembers?: string[]) => GroupDraft;
  transitionType: (group: GroupDraft, type: string) => GroupDraft;
  typeOptions: readonly { value: string; label: string }[];
  validateFilter: (value: unknown) => boolean;
}

export interface StructuredRuleSetDraftAdapter {
  behaviorOptions: (t: Translator) => Array<{ value: string; label: string }>;
  create: (index: number) => RuleSetDraft;
  formatOptions: readonly { value: string; label: string }[];
  formatPatch: (url: string, format: string) => Pick<RuleSetDraft, "format" | "url">;
  fromCatalog: (entry: RuleSetCatalogItem, current: RuleSetDraft[]) => AddCatalogRuleSetResult;
  project: (ruleSets: ConfigMap[]) => RuleSetDraft[] | null;
  serialize: (ruleSets: RuleSetDraft[]) => ConfigMap[];
}

export interface StructuredRuleDraftAdapter {
  create: (index: number, namingLocale?: ConfigNamingLocale) => RuleDraft;
  project: (rules: unknown[]) => RuleDraft[] | null;
  referencesRuleSet: (type: string) => boolean;
  requiresPolicy: (type: string) => boolean;
  requiresValue: (type: string) => boolean;
  serialize: (rules: RuleDraft[]) => unknown[];
  supportsNoResolve: (type: string) => boolean;
  transitionType: (rule: RuleDraft, type: string) => RuleDraft;
  typeOptions: (t: Translator) => Array<{ value: string; label: string }>;
  validateComponent: (value: unknown) => boolean;
}

export interface StructuredFileConfigurationAdapter {
  adaptive: ConfigAdaptiveStrategy;
  kind: string;
  catalogTarget?: RuleSetCatalogTarget;
  decode: (config?: FileConfigDetail, namingLocale?: ConfigNamingLocale) => ConfigEditorDraft | undefined;
  encode: (draft: ConfigEditorDraft) => Record<string, unknown>;
  initialize: (draft?: FileConfigDraft, namingLocale?: ConfigNamingLocale) => ConfigEditorDraft;
  toNativeDraft: (draft: ConfigEditorDraft) => FileConfigDraft;
  groups: StructuredGroupDraftAdapter;
  ruleSets: StructuredRuleSetDraftAdapter;
  rules: StructuredRuleDraftAdapter;
  preview: ConfigPreviewStrategy;
  references: ConfigReferenceStrategy;
  relations: ConfigRelationStrategy;
  templates: ConfigTemplateStrategy;
  validate: (draft: ConfigEditorDraft) => ConfigValidationIssue[];
}

export interface FileProcessorAdapter {
  options?: (t: Translator) => Array<{ value: string; label: string }>;
  addPreset?: (type: string, current: ProcessorDraft[]) => ProcessorDraft[] | undefined;
  normalize: (processors: ProcessorDetail[]) => ProcessorDetail[];
}

interface FileDriverBase {
  kind: string;
  presentation: {
    labelKey: Parameters<Translator>[0];
    icon: FileDriverIcon;
  };
  createPresets: readonly FileCreatePreset[];
  source: {
    defaultBase: (namingLocale: ConfigNamingLocale) => string;
    basePlaceholder: string;
    remoteURLPlaceholder: string;
    syntax: "text" | "yaml" | "json" | "ini";
    strategy: "required" | "optional-base";
    validate: (source: FileSourceDetail) => FileInputValidationCode | null;
  };
  processors: {
    defaults: () => ProcessorDetail[];
    adapter?: FileProcessorAdapter;
    mergeModes: readonly FileMergeMode[];
    validate: (processors: ProcessorDetail[]) => FileProcessorValidationIssue[];
  };
}

export type FileDriverDefinition = FileDriverBase & (
  | { configuration: { mode: "none" } }
  | { configuration: { mode: "raw" } }
  | {
    configuration: {
      mode: "structured";
      adapter: StructuredFileConfigurationAdapter;
      requiresValidOnCreate?: boolean;
    };
  }
);
