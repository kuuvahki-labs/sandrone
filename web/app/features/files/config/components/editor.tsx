import { type ReactNode, useEffect, useMemo, useState } from "react";
import Alert from "@mui/material/Alert";
import Button from "@mui/material/Button";
import Typography from "@mui/material/Typography";

import { adaptiveGenerationDisabledReasonKey } from "~/features/files/config/model/adaptive-availability";
import type { AdaptiveGroupOptions } from "~/features/files/config/model/adaptive-groups";
import {
  type AddCatalogRuleSetRequest,
  type ConfigEditorDraft,
  parseJSONList,
} from "~/features/files/config/model/editor-model";
import {
  applyConfigEditorAdaptiveGeneration,
  applyConfigEditorCatalogRuleSet,
  applyConfigEditorTemplate,
  type ConfigEditorAction,
  deriveConfigEditorOutput,
  deriveConfigEditorValidity,
  initializeConfigEditorState,
  reduceConfigEditorState,
  undoConfigEditorTemplate,
} from "~/features/files/config/model/editor-state";
import type { ConfigNamingLocale } from "~/features/files/config/model/naming";
import type { ConfigTemplateID } from "~/features/files/config/model/templates";
import type { StructuredFileConfigurationAdapter } from "~/features/files/drivers/core/file-driver";
import type { StructuredConfigurationFieldSlots } from "~/features/files/editor/file-driver-ui";
import type { FileConfigDraft } from "~/features/files/model/types";
import { type Translator, useI18n } from "~/shared/i18n/context";
import type { ResourceOption } from "~/shared/resources/types";
import { HighlightedTextarea } from "~/shared/ui/code-editor";

import { ConfigAdaptiveGroupControls } from "./adaptive-group-controls";
import { WorkbenchGroupSection } from "./editor-shared";
import { ProxyGroupEditor } from "./group-editor";
import {
  ConfigNodeSourceSection,
  type ConfigNodeSourceState,
  type LoadSubscriptionPreview,
} from "./node-source";
import { RuleListEditor, RuleSetListEditor } from "./rule-editor";
import { type LoadRuleSetCatalog, RuleSetCatalogDialog } from "./rule-set-catalog-dialog";
import { ConfigTemplateAppliedNotice, type ConfigTemplateChoice, ConfigTemplatePicker, type ConfigTemplatePickerCopy } from "./template-picker";

export interface FileConfigEditorProps {
  adapter: StructuredFileConfigurationAdapter;
  baseEditor: ReactNode;
  createNamingLocale?: ConfigNamingLocale;
  defaultValue?: ConfigEditorDraft | FileConfigDraft;
  mode: "create" | "edit";
  loadSubscriptionPreview?: LoadSubscriptionPreview;
  loadRuleSetCatalog?: LoadRuleSetCatalog;
  onDirty?: () => void;
  onValidityChange?: (valid: boolean) => void;
  subscriptions: ResourceOption[];
  ui: Readonly<StructuredConfigurationFieldSlots>;
}

export function FileConfigEditor({ adapter, baseEditor, createNamingLocale = "en-US", defaultValue, loadRuleSetCatalog, loadSubscriptionPreview = emptySubscriptionPreview, mode: formMode, onDirty, onValidityChange, subscriptions, ui }: FileConfigEditorProps) {
  const { t } = useI18n();
  const [editorState, setEditorState] = useState(() => (
    initializeConfigEditorState(adapter, {
      createNamingLocale,
      defaultValue,
      formMode,
    })
  ));
  const [nodeSourceState, setNodeSourceState] = useState<ConfigNodeSourceState>({
    status: "idle",
    subscriptionName: "",
    preview: null,
  });
  const [catalogOpen, setCatalogOpen] = useState(false);
  const {
    adaptiveOptions,
    adaptiveWarnings,
    namingLocale,
    originalSubscriptions,
    rawSettingsText,
    selectedSubscription: selected,
    settingsMode,
    structure,
    structureRevision,
    templateUndo,
  } = editorState;
  const {
    advancedGroupsText: groupsText,
    advancedRuleSetsText: ruleSetsText,
    advancedRulesText: rulesText,
    editorMode,
    groups,
    ruleSets,
    rules,
  } = structure;
  const rawMode = settingsMode === "raw";
  const currentNodePreview = nodeSourceState.status === "ready"
    && nodeSourceState.subscriptionName === selected
    && nodeSourceState.preview.subscriptionName === selected
    ? nodeSourceState.preview
    : null;
  const nodeOptions = useMemo(() => {
    if (!currentNodePreview) return null;
    return adapter.preview.projectNodes(currentNodePreview);
  }, [adapter, currentNodePreview]);

  const advancedGroups = useMemo(() => parseJSONList(groupsText), [groupsText]);
  const advancedRuleSets = useMemo(() => parseJSONList(ruleSetsText), [ruleSetsText]);
  const advancedRules = useMemo(() => parseJSONList(rulesText), [rulesText]);
  const output = useMemo(
    () => deriveConfigEditorOutput(adapter, editorState),
    [adapter, editorState],
  );
  const validity = useMemo(() => deriveConfigEditorValidity(
    adapter,
    editorState,
    output,
    {
      currentPreview: currentNodePreview,
      projectedNodes: nodeOptions,
    },
  ), [adapter, currentNodePreview, editorState, nodeOptions, output]);
  const {
    effectiveAdaptiveConfig,
    multipleSubscriptions,
    nativeConfig: configValue,
    rawSettingsError,
    serialized,
  } = output;
  const {
    adaptiveStale,
    previewValidation,
    relationModel,
    valid,
  } = validity;
  const recognition = useMemo(
    () => adapter.templates.recognize(configValue),
    [adapter, configValue],
  );
  const recognizedTemplate = recognition.match && recognition.match !== "custom"
    ? recognition.match
    : null;
  const copy = useMemo(() => workbenchCopy(t), [t]);
  const templates = useMemo<ConfigTemplateChoice[]>(() => (
    adapter.templates.list().map((template) => ({
      ...template,
      name: copy.templateNames[template.id],
      description: copy.templateDescriptions[template.id],
    }))
  ), [adapter, copy.templateDescriptions, copy.templateNames]);
  const adaptiveCandidates = useMemo(() => adapter.adaptive.generate(
    nodeOptions?.map((node) => node.name) ?? [],
    adaptiveOptions,
    namingLocale,
  ).candidates, [adapter, adaptiveOptions, namingLocale, nodeOptions]);
  const adaptiveDisabledReasonKey = adaptiveGenerationDisabledReasonKey({
    anchorProblem: adapter.adaptive.anchorProblem(effectiveAdaptiveConfig),
    editorMode,
    hasCurrentPreview: Boolean(currentNodePreview),
    nodeCount: nodeOptions?.length ?? 0,
    previewStatus: nodeSourceState.status,
    requiresNodePreview: adapter.adaptive.requiresNodePreview,
    selected: Boolean(selected),
  });
  const adaptiveDisabledReason = adaptiveDisabledReasonKey
    ? t(adaptiveDisabledReasonKey)
    : undefined;
  const templateSummary = recognizedTemplate && recognition.adaptive
    ? t("files.config.templateWithAdaptive", { template: copy.templateNames[recognizedTemplate] })
    : undefined;
  const templatePickerCopy = recognition.adaptive
    ? { ...copy.templatePicker, dialogDescription: t("files.config.templateApplyAdaptiveDescription") }
    : copy.templatePicker;

  useEffect(() => onValidityChange?.(valid), [onValidityChange, valid]);

  function applyTemplate(choice: ConfigTemplateChoice) {
    onDirty?.();
    setEditorState((current) => applyConfigEditorTemplate(
      adapter,
      current,
      choice.id as ConfigTemplateID,
    ));
  }

  function undoTemplate() {
    if (!templateUndo) return;
    onDirty?.();
    setEditorState(undoConfigEditorTemplate(editorState));
  }

  function generateAdaptive(options: AdaptiveGroupOptions) {
    const transition = applyConfigEditorAdaptiveGeneration(
      adapter,
      editorState,
      {
        nodeNames: nodeOptions?.map((node) => node.name) ?? [],
        options,
      },
    );
    if (!transition.applied) return;
    onDirty?.();
    setEditorState(transition.state);
  }

  function addFromCatalog(request: AddCatalogRuleSetRequest) {
    const transition = applyConfigEditorCatalogRuleSet(
      adapter,
      editorState,
      request,
    );
    if (transition.result.status === "added") {
      onDirty?.();
      setEditorState(transition.state);
    }
    return transition.result;
  }

  function updateEditorState(event: ConfigEditorAction) {
    setEditorState((current) => reduceConfigEditorState(current, event));
  }

  const groupIssues = relationModel.issues.filter((issue) => issue.section === "groups");
  const ruleSetIssues = relationModel.issues.filter((issue) => issue.section === "rule_sets");
  const ruleIssues = relationModel.issues.filter((issue) => issue.section === "rules");

  return (
    <div className="grid min-w-0 gap-3">
      <input name="config" type="hidden" value={serialized} />
      <Typography className="font-semibold" component="h2" variant="h6">
        {t("files.config.content")}
      </Typography>
			{multipleSubscriptions ? <Alert severity="error">{t("files.config.multipleSubscriptions", { names: originalSubscriptions.join(", ") })}</Alert> : null}
      {!rawMode ? (
        <WorkbenchGroupSection collapsible={false} id="config-templates" label={copy.templateSection} summary={templateSummary}>
          <ConfigTemplatePicker
            choices={templates}
            confirmBeforeApply={formMode === "edit" || recognition.adaptive}
            copy={templatePickerCopy}
            currentTemplateId={recognizedTemplate ?? undefined}
            customized={recognition.match === "custom"}
            labelledBy="config-templates-header"
            onRequestApply={applyTemplate}
          />
          {templateUndo ? <ConfigTemplateAppliedNotice message={copy.applied} undoLabel={copy.undo} onUndo={undoTemplate} /> : null}
        </WorkbenchGroupSection>
      ) : null}
      <ConfigNodeSourceSection
        disabled={multipleSubscriptions}
        loadPreview={loadSubscriptionPreview}
        selected={selected}
        subscriptions={subscriptions}
        onSelectedChange={(name) => {
          onDirty?.();
          updateEditorState({ type: "select-subscription", name });
        }}
        onStateChange={setNodeSourceState}
      />
			{previewValidation.issueKey ? <Alert severity="error">{t(previewValidation.issueKey)}</Alert> : null}
			{rawMode ? (
				<>
					<Alert severity="warning">{t("files.config.rawSettingsPreserved")}</Alert>
					<WorkbenchGroupSection collapsible={false} id="file-config-base" label={t("files.config.baseContent")}>
						{baseEditor}
					</WorkbenchGroupSection>
					<HighlightedTextarea label={t("files.config.rawSettings")} language="json" minRows={12} showLineNumbers value={rawSettingsText} onChange={(event) => {
            updateEditorState({
              type: "edit-raw-settings",
              text: event.target.value,
            });
            onDirty?.();
          }} />
					{rawSettingsError ? <Alert severity="error">{t("files.config.rawSettingsInvalid")}</Alert> : null}
						<Button type="button" variant="outlined" onClick={() => {
							if (window.confirm(t("files.config.replaceRawSettingsConfirm"))) {
								updateEditorState({ type: "replace-raw-with-structured" });
								onDirty?.();
						}
					}}>{t("files.config.replaceRawSettings")}</Button>
				</>
			) : (
				<>
      <ConfigAdaptiveGroupControls
        candidates={adaptiveCandidates}
        disabledReason={adaptiveDisabledReason}
		generatedCount={adapter.adaptive.canonicalNames(effectiveAdaptiveConfig.groups ?? []).length}
		options={adaptiveOptions}
		typeOptions={adapter.adaptive.typeOptions}
        warnings={adaptiveWarnings}
        onOptionsChange={(options) => {
          onDirty?.();
          updateEditorState({ type: "change-adaptive-options", options });
        }}
        onGenerate={generateAdaptive}
      />
      {adaptiveStale ? <Alert severity="error">{t("files.config.adaptiveStale")}</Alert> : null}
      <WorkbenchGroupSection collapsible={false} id="file-config-base" label={t("files.config.baseContent")}>
        {baseEditor}
      </WorkbenchGroupSection>
      {editorMode === "advanced" ? <Alert severity="warning"><Typography className="font-semibold" component="p" variant="body2">{t("files.config.rawConfig")}</Typography>{t("files.config.advancedUnsupported")}</Alert> : null}
      {editorMode === "wizard" ? (
        <>
          <ProxyGroupEditor adapter={adapter} defaultExpanded groups={groups} inboundReferences={relationModel.groupInboundReferences} issues={groupIssues} key={`groups-${structureRevision}`} namingLocale={namingLocale} nodes={nodeOptions ?? []} ui={ui} onChange={(value) => updateEditorState({ type: "change-groups", groups: value })} />
          <RuleSetListEditor adapter={adapter} defaultExpanded={ruleSets.length <= 20} inboundReferences={relationModel.ruleSetInboundReferences} issues={ruleSetIssues} key={`rule-sets-${structureRevision}`} ruleSets={ruleSets} ui={ui} onChange={(value) => updateEditorState({ type: "change-rule-sets", ruleSets: value })} onOpenCatalog={loadRuleSetCatalog ? () => setCatalogOpen(true) : undefined} />
          <RuleListEditor adapter={adapter} groups={groups} issues={ruleIssues} key={`rules-${structureRevision}`} namingLocale={namingLocale} nodes={nodeOptions ?? []} rules={rules} ruleSets={ruleSets} ui={ui} onChange={(value) => updateEditorState({ type: "change-rules", rules: value })} />
        </>
      ) : (
        <>
          <RawListSection defaultExpanded error={advancedGroups.error} id="config-proxy-groups" label={t("files.config.proxyGroups")} textLabel={t("files.config.groupsRaw")} value={groupsText} onChange={(text) => updateEditorState({ type: "change-advanced-groups", text })} />
          <RawListSection defaultExpanded error={advancedRuleSets.error} id="config-rule-sets" label={t("files.config.ruleSets")} textLabel={t("files.config.ruleSetsRaw")} value={ruleSetsText} onChange={(text) => updateEditorState({ type: "change-advanced-rule-sets", text })} />
          <RawListSection defaultExpanded error={advancedRules.error} id="config-routing-rules" label={t("files.config.rules")} textLabel={t("files.config.rulesRaw")} value={rulesText} onChange={(text) => updateEditorState({ type: "change-advanced-rules", text })} />
        </>
      )}
      {loadRuleSetCatalog && adapter.catalogTarget ? (
        <RuleSetCatalogDialog
          kind={adapter.catalogTarget}
          loadCatalog={loadRuleSetCatalog}
          open={catalogOpen}
          onAdd={addFromCatalog}
          onClose={() => setCatalogOpen(false)}
        />
      ) : null}
				</>
			)}
    </div>
  );
}

const emptySubscriptionPreview: LoadSubscriptionPreview = async (name: string) => ({
  subscriptionName: name,
  nodes: [],
  warnings: [],
});

function RawListSection({ defaultExpanded, error, id, label, onChange, textLabel, value }: {
  defaultExpanded?: boolean; error?: string; id: string; label: string; onChange: (value: string) => void; textLabel: string; value: string;
}) {
  const { t } = useI18n();
  return (
    <WorkbenchGroupSection defaultExpanded={defaultExpanded} id={id} label={label} severity={error ? "error" : "success"} severityLabel={error ? t("files.config.jsonInvalid") : t("files.config.statusValid")}>
      <HighlightedTextarea showLineNumbers label={textLabel} language="json" minRows={6} value={value} onChange={(event) => onChange(event.target.value)} />
      {error ? <Typography color="error" role="alert" variant="caption">{t("files.config.invalidJsonArray")}</Typography> : null}
    </WorkbenchGroupSection>
  );
}

function workbenchCopy(t: Translator) {
  const templateNames: Record<ConfigTemplateID, string> = {
    minimal: t("files.config.templateMinimal"),
    standard: t("files.config.templateStandard"),
    full: t("files.config.templateFull"),
  };
  const templateDescriptions: Record<ConfigTemplateID, string> = {
    minimal: t("files.config.templateMinimalDescription"),
    standard: t("files.config.templateStandardDescription"),
    full: t("files.config.templateFullDescription"),
  };
  const templatePicker: Partial<ConfigTemplatePickerCopy> = {
    cancel: t("actions.cancel"),
    confirm: t("actions.replace"),
    confirmAccessibleLabel: t("files.config.templateApplyConfirm"),
    customized: t("files.config.templateCustomized"),
    dialogDescription: t("files.config.templateApplyDescription"),
    dialogTitle: t("files.config.templateApplyTitle"),
    groups: (count) => t("files.config.templateCountGroups", { count }),
    label: t("files.config.templateLabel"),
    ruleSets: (count) => t("files.config.templateCountRuleSets", { count }),
    rules: (count) => t("files.config.templateCountRules", { count }),
  };
  return {
    applied: t("files.config.templateApplied"),
    customized: t("files.config.templateCustomized"),
    templateDescriptions,
    templateNames,
    templatePicker,
    templateSection: t("files.config.templateLabel"),
    undo: t("files.config.templateUndo"),
  };
}
