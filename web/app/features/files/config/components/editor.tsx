import { type ReactNode, useEffect, useMemo, useState } from "react";
import Alert from "@mui/material/Alert";
import Button from "@mui/material/Button";
import Typography from "@mui/material/Typography";

import { adaptiveGenerationDisabledReasonKey } from "~/features/files/config/model/adaptive-availability";
import {
	type AdaptiveGroupOptions,
	type AdaptiveGroupWarning,
} from "~/features/files/config/model/adaptive-groups";
import {
  type AddCatalogRuleSetRequest,
  type ConfigEditorDraft,
  type GroupDraft,
  isRecord,
  parseJSONList,
  type RuleDraft,
  type RuleSetDraft,
  type StructureSectionPresence,
} from "~/features/files/config/model/editor-model";
import {
  type ConfigNamingLocale,
} from "~/features/files/config/model/naming";
import { buildConfigRelationModel } from "~/features/files/config/model/relations";
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

interface StructureSnapshot {
  advancedGroupsText: string;
  advancedRuleSetsText: string;
  advancedRulesText: string;
  editorMode: "wizard" | "advanced";
  groupPreset: string;
  groups: GroupDraft[];
  ruleSetPreset: string;
  ruleSets: RuleSetDraft[];
  rules: RuleDraft[];
  sectionPresence: StructureSectionPresence;
}

export function FileConfigEditor({ adapter, baseEditor, createNamingLocale = "en-US", defaultValue, loadRuleSetCatalog, loadSubscriptionPreview = emptySubscriptionPreview, mode: formMode, onDirty, onValidityChange, subscriptions, ui }: FileConfigEditorProps) {
	const { t } = useI18n();
	const nativeDefault = useMemo(() => isEditorDraft(defaultValue)
		? adapter.toNativeDraft(defaultValue)
		: defaultValue, [adapter, defaultValue]);
	const [namingLocale] = useState<ConfigNamingLocale>(() => (
		adapter.templates.resolveNamingLocale(nativeDefault, createNamingLocale)
	));
	const initial = useMemo(
		() => isEditorDraft(defaultValue)
			? defaultValue
			: adapter.initialize(defaultValue ?? adapter.templates.create("standard", namingLocale), namingLocale),
		[adapter, defaultValue, namingLocale],
  );
	const originalSubscriptions = useMemo(() => initial.subscriptions ?? [], [initial.subscriptions]);
	const multipleSubscriptions = originalSubscriptions.length > 1;
  const [selected, setSelected] = useState(() => originalSubscriptions.length === 1 ? originalSubscriptions[0] : "");
	const [rawMode, setRawMode] = useState(initial.settingsMode === "raw");
	const [rawSettingsText, setRawSettingsText] = useState(() => JSON.stringify(initial.rawSettings ?? {}, null, 2));
  const [nodeSourceState, setNodeSourceState] = useState<ConfigNodeSourceState>({
    status: "idle",
    subscriptionName: "",
    preview: null,
  });
  const [editorMode, setEditorMode] = useState(initial.mode);
  const [groupPreset, setGroupPreset] = useState(initial.groupPreset);
  const [ruleSetPreset, setRuleSetPreset] = useState(initial.ruleSetPreset);
  const [groups, setGroups] = useState(initial.groups);
  const [ruleSets, setRuleSets] = useState(initial.ruleSets);
  const [rules, setRules] = useState(initial.rules);
  const [sectionPresence, setSectionPresence] = useState<StructureSectionPresence>(initial.sectionPresence);
  const [catalogOpen, setCatalogOpen] = useState(false);
  const [groupsText, setGroupsText] = useState(initial.advancedGroupsText);
  const [ruleSetsText, setRuleSetsText] = useState(initial.advancedRuleSetsText);
  const [rulesText, setRulesText] = useState(initial.advancedRulesText);
  const [structureRevision, setStructureRevision] = useState(0);
  const [undoSnapshot, setUndoSnapshot] = useState<StructureSnapshot | null>(null);
  const [appliedTemplateName, setAppliedTemplateName] = useState("");
	const [adaptiveWarnings, setAdaptiveWarnings] = useState<AdaptiveGroupWarning[]>([]);
	const [adaptiveEnabled, setAdaptiveEnabled] = useState(
		adapter.adaptive.initiallyEnabled(formMode, initial.adaptiveGroups),
	);
	const [adaptiveOptionsChanged, setAdaptiveOptionsChanged] = useState(formMode === "create");
	const [adaptiveOptions, setAdaptiveOptions] = useState<AdaptiveGroupOptions>(() => (
		adapter.adaptive.optionsFromConfig(initial.adaptiveGroups)
	));
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
	const rawSettings = useMemo(() => parseJSONObject(rawSettingsText), [rawSettingsText]);
  const serializedGroups = useMemo(() => adapter.groups.serialize(groups), [adapter, groups]);
  const serializedRuleSets = useMemo(() => adapter.ruleSets.serialize(ruleSets), [adapter, ruleSets]);
  const serializedRules = useMemo(() => adapter.rules.serialize(rules), [adapter, rules]);
  const currentDraft = useMemo<ConfigEditorDraft>(() => ({
    subscriptions: [],
    settingsMode: "structured",
    rawSettings: initial.rawSettings,
		adaptiveGroups: adaptiveEnabled
			? adaptiveOptionsChanged
				? adapter.adaptive.configFromOptions(adaptiveOptions)
				: initial.adaptiveGroups
			: undefined,
    advancedGroupsText: groupsText,
    advancedRuleSetsText: ruleSetsText,
    advancedRulesText: rulesText,
    groupPreset,
    groups,
    mode: editorMode,
    ruleSetPreset,
    ruleSets,
    rules,
    sectionPresence,
	}), [adapter, adaptiveEnabled, adaptiveOptions, adaptiveOptionsChanged, editorMode, groupPreset, groups, groupsText, initial.adaptiveGroups, initial.rawSettings, ruleSetPreset, ruleSets, ruleSetsText, rules, rulesText, sectionPresence]);
	const relationModel = useMemo(() => {
		if (editorMode !== "wizard") return { groupInboundReferences: {}, ruleSetInboundReferences: {}, issues: [] };
		const nodeNames = adapter.preview.relationNodeNames(nodeOptions, Boolean(selected));
		const base = buildConfigRelationModel(adapter.relations.project(
			serializedGroups,
			serializedRuleSets,
			serializedRules,
			nodeNames,
		));
		return { ...base, issues: [...base.issues, ...adapter.validate(currentDraft)] };
	}, [adapter, currentDraft, editorMode, nodeOptions, selected, serializedGroups, serializedRuleSets, serializedRules]);

  const envelopeSubscriptions = useMemo(
    () => multipleSubscriptions ? originalSubscriptions : selected ? [selected] : [],
    [multipleSubscriptions, originalSubscriptions, selected],
  );
  const configValue = useMemo<FileConfigDraft>(() => adapter.toNativeDraft({
    ...currentDraft,
    subscriptions: envelopeSubscriptions,
  }), [adapter, currentDraft, envelopeSubscriptions]);
	const effectiveAdaptiveConfig = useMemo<FileConfigDraft>(() => (
		sectionPresence.groups ? configValue : { ...configValue, groups: serializedGroups }
	), [configValue, sectionPresence.groups, serializedGroups]);
	const serialized = useMemo(() => JSON.stringify(adapter.encode({
		...currentDraft,
		subscriptions: envelopeSubscriptions,
		settingsMode: rawMode ? "raw" : "structured",
		rawSettings: rawSettings.value ?? initial.rawSettings ?? {},
	})), [adapter, currentDraft, envelopeSubscriptions, initial.rawSettings, rawMode, rawSettings.value]);
	const recognition = useMemo(() => adapter.templates.recognize(configValue), [adapter, configValue]);
  const recognizedTemplate = recognition.match && recognition.match !== "custom"
    ? recognition.match
    : null;
  const copy = useMemo(() => workbenchCopy(t), [t]);
	const templates = useMemo<ConfigTemplateChoice[]>(() => adapter.templates.list().map((template) => ({
		...template,
		name: copy.templateNames[template.id],
		description: copy.templateDescriptions[template.id],
	})), [adapter, copy.templateDescriptions, copy.templateNames]);
	const adaptiveCandidates = useMemo(() => adapter.adaptive.generate(
		nodeOptions?.map((node) => node.name) ?? [],
		adaptiveOptions,
		namingLocale,
	).candidates, [adapter, adaptiveOptions, namingLocale, nodeOptions]);
	const adaptiveStale = useMemo(() => adapter.adaptive.isStale({
		config: effectiveAdaptiveConfig,
		editorMode,
		enabled: adaptiveEnabled,
		namingLocale,
		nodeNames: nodeOptions?.map((node) => node.name),
		options: adaptiveOptions,
	}), [adapter, adaptiveEnabled, adaptiveOptions, editorMode, effectiveAdaptiveConfig, namingLocale, nodeOptions]);
	const previewValidation = adapter.preview.validate({
		formMode,
		preview: currentNodePreview,
		projectedNodes: nodeOptions,
		selected: Boolean(selected),
	});
  const structureValid = editorMode === "advanced"
    ? !advancedGroups.error && !advancedRuleSets.error && !advancedRules.error
    : !relationModel.issues.some((issue) => issue.severity === "error");
	const valid = !multipleSubscriptions && previewValidation.valid && (rawMode ? !rawSettings.error : structureValid && !adaptiveStale);
	const adaptiveDisabledReasonKey = adaptiveGenerationDisabledReasonKey({
		anchorProblem: adapter.adaptive.anchorProblem(effectiveAdaptiveConfig),
    editorMode,
    hasCurrentPreview: Boolean(currentNodePreview),
    nodeCount: nodeOptions?.length ?? 0,
		previewStatus: nodeSourceState.status,
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
    setUndoSnapshot(captureSnapshot());
		const nextConfig = adapter.templates.create(choice.id as ConfigTemplateID, namingLocale);
    const next = adapter.initialize(nextConfig, namingLocale);
    setEditorMode(next.mode);
    setGroupPreset(next.groupPreset);
    setRuleSetPreset(next.ruleSetPreset);
    setGroups(next.groups);
    setRuleSets(next.ruleSets);
    setRules(next.rules);
    setGroupsText(next.advancedGroupsText);
    setRuleSetsText(next.advancedRuleSetsText);
    setRulesText(next.advancedRulesText);
    setSectionPresence({ groups: true, ruleSets: true, rules: true });
    setStructureRevision((current) => current + 1);
    setAppliedTemplateName(choice.name);
    setAdaptiveWarnings([]);
  }

  function undoTemplate() {
    if (!undoSnapshot) return;
    onDirty?.();
    setEditorMode(undoSnapshot.editorMode);
    setGroupPreset(undoSnapshot.groupPreset);
    setRuleSetPreset(undoSnapshot.ruleSetPreset);
    setGroups(undoSnapshot.groups);
    setRuleSets(undoSnapshot.ruleSets);
    setRules(undoSnapshot.rules);
    setGroupsText(undoSnapshot.advancedGroupsText);
    setRuleSetsText(undoSnapshot.advancedRuleSetsText);
    setRulesText(undoSnapshot.advancedRulesText);
    setSectionPresence(undoSnapshot.sectionPresence);
    setStructureRevision((current) => current + 1);
    setUndoSnapshot(null);
    setAppliedTemplateName("");
    setAdaptiveWarnings([]);
  }

  function generateAdaptive(options: AdaptiveGroupOptions) {
    if (!nodeOptions) return;
		const generation = adapter.adaptive.generate(
			nodeOptions.map((node) => node.name),
			options,
			namingLocale,
		);
		const result = adapter.adaptive.merge(effectiveAdaptiveConfig, generation);
    const projectedGroups = adapter.groups.project(result.config.groups ?? []);
    if (!projectedGroups) return;
    onDirty?.();
    setAdaptiveEnabled(true);
    setAdaptiveOptionsChanged(true);
    setAdaptiveOptions(options);
    setGroups(projectedGroups);
    setSectionPresence((current) => ({ ...current, groups: true }));
    setAdaptiveWarnings(result.warnings);
    setStructureRevision((current) => current + 1);
    setUndoSnapshot(null);
    setAppliedTemplateName("");
  }

  function captureSnapshot(): StructureSnapshot {
    return { advancedGroupsText: groupsText, advancedRuleSetsText: ruleSetsText, advancedRulesText: rulesText, editorMode, groupPreset, groups, ruleSetPreset, ruleSets, rules, sectionPresence };
  }

  function addFromCatalog(request: AddCatalogRuleSetRequest) {
    const result = adapter.ruleSets.fromCatalog(request.entry, ruleSets);
    if (result.status === "added") {
      onDirty?.();
      setRuleSets(result.ruleSets);
      setSectionPresence((current) => ({ ...current, ruleSets: true }));
      setUndoSnapshot(null);
      setAppliedTemplateName("");
    }
    return result;
  }

  const groupIssues = relationModel.issues.filter((issue) => issue.section === "groups");
  const ruleSetIssues = relationModel.issues.filter((issue) => issue.section === "rule_sets");
  const ruleIssues = relationModel.issues.filter((issue) => issue.section === "rules");

  return (
    <div className="grid min-w-0 gap-3">
      <input name="config" type="hidden" value={serialized} />
			{multipleSubscriptions ? <Alert severity="error">{t("files.config.multipleSubscriptions", { names: originalSubscriptions.join(", ") })}</Alert> : null}
      <ConfigNodeSourceSection
        disabled={multipleSubscriptions}
        loadPreview={loadSubscriptionPreview}
        selected={selected}
        subscriptions={subscriptions}
        onSelectedChange={(name) => {
          onDirty?.();
          setSelected(name);
        }}
        onStateChange={setNodeSourceState}
      />
			{previewValidation.issueKey ? <Alert severity="error">{t(previewValidation.issueKey)}</Alert> : null}
			{rawMode ? (
				<>
					<Alert severity="warning">{t("files.config.rawSettingsPreserved")}</Alert>
					<WorkbenchGroupSection collapsible={false} id="file-config-base" label={t("files.config.base")}>
						{baseEditor}
					</WorkbenchGroupSection>
					<HighlightedTextarea label={t("files.config.rawSettings")} language="json" minRows={12} showLineNumbers value={rawSettingsText} onChange={(event) => {
            setRawSettingsText(event.target.value);
            onDirty?.();
          }} />
					{rawSettings.error ? <Alert severity="error">{t("files.config.rawSettingsInvalid")}</Alert> : null}
						<Button type="button" variant="outlined" onClick={() => {
							if (window.confirm(t("files.config.replaceRawSettingsConfirm"))) {
								setRawMode(false);
								setSectionPresence({ groups: true, ruleSets: true, rules: true });
								onDirty?.();
						}
					}}>{t("files.config.replaceRawSettings")}</Button>
				</>
			) : (
				<>
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
        {appliedTemplateName ? <ConfigTemplateAppliedNotice message={copy.applied} undoLabel={copy.undo} onUndo={undoTemplate} /> : null}
      </WorkbenchGroupSection>
      <ConfigAdaptiveGroupControls
        candidates={adaptiveCandidates}
        disabledReason={adaptiveDisabledReason}
		generatedCount={adapter.adaptive.canonicalNames(effectiveAdaptiveConfig.groups ?? []).length}
		options={adaptiveOptions}
		typeOptions={adapter.adaptive.typeOptions}
        warnings={adaptiveWarnings}
        onOptionsChange={(options) => {
          onDirty?.();
          setAdaptiveEnabled(true);
          setAdaptiveOptionsChanged(true);
          setAdaptiveOptions(options);
          setAdaptiveWarnings([]);
        }}
        onGenerate={generateAdaptive}
      />
      {adaptiveStale ? <Alert severity="error">{t("files.config.adaptiveStale")}</Alert> : null}
      <Typography className="font-semibold" component="h3" id="file-config-details-heading" variant="subtitle1">
        {t("files.config.details")}
      </Typography>
      <WorkbenchGroupSection collapsible={false} id="file-config-base" label={t("files.config.base")}>
        {baseEditor}
      </WorkbenchGroupSection>
      {editorMode === "advanced" ? <Alert severity="warning"><Typography className="font-semibold" component="p" variant="body2">{t("files.config.rawConfig")}</Typography>{t("files.config.advancedUnsupported")}</Alert> : null}
      {editorMode === "wizard" ? (
        <>
          <ProxyGroupEditor adapter={adapter} defaultExpanded groups={groups} inboundReferences={relationModel.groupInboundReferences} issues={groupIssues} key={`groups-${structureRevision}`} namingLocale={namingLocale} nodes={nodeOptions ?? []} ui={ui} onChange={(value) => { setGroups(value); setSectionPresence((current) => ({ ...current, groups: true })); }} />
          <RuleSetListEditor adapter={adapter} defaultExpanded={ruleSets.length <= 20} inboundReferences={relationModel.ruleSetInboundReferences} issues={ruleSetIssues} key={`rule-sets-${structureRevision}`} ruleSets={ruleSets} ui={ui} onChange={(value) => { setRuleSets(value); setSectionPresence((current) => ({ ...current, ruleSets: true })); }} onOpenCatalog={loadRuleSetCatalog ? () => setCatalogOpen(true) : undefined} />
          <RuleListEditor adapter={adapter} groups={groups} issues={ruleIssues} key={`rules-${structureRevision}`} namingLocale={namingLocale} nodes={nodeOptions ?? []} rules={rules} ruleSets={ruleSets} ui={ui} onChange={(value) => { setRules(value); setSectionPresence((current) => ({ ...current, rules: true })); }} />
        </>
      ) : (
        <>
          <RawListSection defaultExpanded error={advancedGroups.error} id="config-proxy-groups" label={t("files.config.proxyGroups")} textLabel={t("files.config.groupsRaw")} value={groupsText} onChange={(value) => { setGroupsText(value); setSectionPresence((current) => ({ ...current, groups: true })); }} />
          <RawListSection defaultExpanded error={advancedRuleSets.error} id="config-rule-sets" label={t("files.config.ruleSets")} textLabel={t("files.config.ruleSetsRaw")} value={ruleSetsText} onChange={(value) => { setRuleSetsText(value); setSectionPresence((current) => ({ ...current, ruleSets: true })); }} />
          <RawListSection defaultExpanded error={advancedRules.error} id="config-routing-rules" label={t("files.config.rules")} textLabel={t("files.config.rulesRaw")} value={rulesText} onChange={(value) => { setRulesText(value); setSectionPresence((current) => ({ ...current, rules: true })); }} />
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

function parseJSONObject(text: string): { error?: string; value?: Record<string, unknown> } {
	try {
		const value = JSON.parse(text) as unknown;
		return isRecord(value) ? { value } : { error: "not-object" };
	} catch {
		return { error: "invalid-json" };
	}
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

function isEditorDraft(value: ConfigEditorDraft | FileConfigDraft | undefined): value is ConfigEditorDraft {
  return Boolean(value && "sectionPresence" in value && "advancedGroupsText" in value);
}
