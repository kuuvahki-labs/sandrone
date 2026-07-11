import { useState } from "react";
import { closestCenter, DndContext, type DragEndEvent } from "@dnd-kit/core";
import { arrayMove, SortableContext, useSortable, verticalListSortingStrategy } from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import AddIcon from "@mui/icons-material/Add";
import DeleteOutlinedIcon from "@mui/icons-material/DeleteOutlined";
import DragIndicatorIcon from "@mui/icons-material/DragIndicator";
import KeyboardArrowDownIcon from "@mui/icons-material/KeyboardArrowDown";
import KeyboardArrowRightIcon from "@mui/icons-material/KeyboardArrowRight";
import Button from "@mui/material/Button";
import Collapse from "@mui/material/Collapse";
import IconButton from "@mui/material/IconButton";
import TextField from "@mui/material/TextField";
import ToggleButton from "@mui/material/ToggleButton";
import ToggleButtonGroup from "@mui/material/ToggleButtonGroup";
import Tooltip from "@mui/material/Tooltip";
import Typography from "@mui/material/Typography";

import {
  type GroupDraft,
  type RuleDraft,
  type RuleSetDraft,
} from "~/features/files/config/model/editor-model";
import type { ConfigNamingLocale } from "~/features/files/config/model/naming";
import type { ConfigNodeSummary } from "~/features/files/config/model/node-source";
import { policyReferenceOptions, ruleSetReferenceOptions } from "~/features/files/config/model/references";
import type { ConfigValidationIssue } from "~/features/files/config/model/relations";
import type { StructuredFileConfigurationAdapter } from "~/features/files/drivers/core/file-driver";
import type {
  RuleSetHeaderLayout,
  StructuredConfigurationFieldSlots,
} from "~/features/files/editor/file-driver-ui";
import { useI18n } from "~/shared/i18n/context";
import { SelectField } from "~/shared/ui/form-fields";
import { ActionMenu } from "~/shared/ui/resource-list";

import {
  configEditorPanelClassName,
  ConfigRowSummary,
  DenseConfigRow,
  issueSummary,
  removeAt,
  replaceAt,
  RowOrderActions,
  SectionIssues,
  severityForIssues,
  useSortableSensors,
  WorkbenchGroupSection,
} from "./editor-shared";
import { CreatableReferenceField } from "./reference-fields";

export function RuleSetListEditor({ adapter, defaultExpanded = true, inboundReferences, issues, onChange, onOpenCatalog, ruleSets, ui }: {
  adapter: StructuredFileConfigurationAdapter;
  defaultExpanded?: boolean;
  inboundReferences: Record<string, number>;
  issues: ConfigValidationIssue[];
  onChange: (value: RuleSetDraft[]) => void;
  onOpenCatalog?: () => void;
  ruleSets: RuleSetDraft[];
  ui: Readonly<StructuredConfigurationFieldSlots>;
}) {
  const { t } = useI18n();
  const [openRuleSetID, setOpenRuleSetID] = useState<string | null>(null);
  const update = (index: number, patch: Partial<RuleSetDraft>) => onChange(replaceAt(ruleSets, index, { ...ruleSets[index], ...patch }));
  return (
    <WorkbenchGroupSection
      count={ruleSets.length}
      defaultExpanded={defaultExpanded}
      headerActions={(
        <>
          {onOpenCatalog ? <Button aria-label={t("files.config.catalogOpen")} type="button" variant="outlined" onClick={onOpenCatalog}>{t("files.config.catalogOpenShort")}</Button> : null}
          <Button aria-label={t("files.config.addRuleSet")} startIcon={<AddIcon aria-hidden />} type="button" variant="outlined" onClick={() => onChange([...ruleSets, adapter.ruleSets.create(ruleSets.length)])}>{t("actions.add")}</Button>
        </>
      )}
      id="config-rule-sets"
      label={t("files.config.ruleSets")}
      severity={severityForIssues(issues)}
      severityLabel={issues.length ? t("files.config.statusNeedsAttention") : t("files.config.statusValid")}
      summary={issueSummary(issues, t, t("files.config.inboundTotal", { count: Object.values(inboundReferences).reduce((sum, count) => sum + count, 0) }))}
    >
      <SectionIssues issues={issues} />
      <div className="overflow-hidden rounded-md border border-divider">
        {ruleSets.map((ruleSet, index) => (
          <RuleSetRow
            inboundCount={inboundReferences[ruleSet.name.trim()] ?? 0}
            index={index}
            key={ruleSet.id}
            adapter={adapter}
            open={openRuleSetID === ruleSet.id}
            ruleSet={ruleSet}
            ui={ui}
            onDelete={() => {
              onChange(removeAt(ruleSets, index));
              setOpenRuleSetID(null);
            }}
            onOpenChange={() => setOpenRuleSetID((current) => current === ruleSet.id ? null : ruleSet.id)}
            onUpdate={(patch) => update(index, patch)}
          />
        ))}
      </div>
    </WorkbenchGroupSection>
  );
}

function RuleSetRow({ adapter, inboundCount, index, onDelete, onOpenChange, onUpdate, open, ruleSet, ui }: {
  adapter: StructuredFileConfigurationAdapter;
  inboundCount: number;
  index: number;
  onDelete: () => void;
  onOpenChange: () => void;
  onUpdate: (patch: Partial<RuleSetDraft>) => void;
  open: boolean;
  ruleSet: RuleSetDraft;
  ui: Readonly<StructuredConfigurationFieldSlots>;
}) {
  const { t } = useI18n();
  const name = ruleSet.name || t("files.config.ruleSet");
  const accessibleLabel = ruleSet.name ? `${index + 1} ${ruleSet.name}` : String(index + 1);
  const contentID = `rule-set-${ruleSet.id}-editor`;
  const behaviorOptions = adapter.ruleSets.behaviorOptions(t);
  const behaviorLabel = behaviorOptions.find((option) => option.value === ruleSet.behavior)?.label ?? ruleSet.behavior;
  const presentation = ui.ruleSetPresentation;
  const remote = presentation.sourceMode === "remote-only" || ruleSet.source === "remote";
  return (
    <div aria-label={`${t("files.config.ruleSet")} ${accessibleLabel}`} className="border-t border-divider bg-background-paper first:border-t-0" role="group">
      <DenseConfigRow>
        <IconButton aria-controls={open ? contentID : undefined} aria-expanded={open} aria-label={`${open ? t("files.config.collapseRuleSet") : t("files.config.expandRuleSet")} ${accessibleLabel}`} size="small" type="button" onClick={onOpenChange}>
          {open ? <KeyboardArrowDownIcon aria-hidden fontSize="small" /> : <KeyboardArrowRightIcon aria-hidden fontSize="small" />}
        </IconButton>
        <ConfigRowSummary
          primary={name}
          secondary={[
            remote ? t("files.config.url") : t("files.config.ruleSetSourceInline"),
            ...(presentation.summaryFields.includes("behavior") ? [behaviorLabel || t("files.config.behaviorClassical")] : []),
            ...(remote && presentation.summaryFields.includes("format") ? [ruleSet.format] : []),
            t("files.config.inboundCount", { count: inboundCount }),
          ]}
        />
        <div className="hidden sm:block">
          <Tooltip title={t("files.config.deleteRuleSet")}><IconButton aria-label={`${t("files.config.deleteRuleSet")} ${accessibleLabel}`} size="small" type="button" onClick={onDelete}><DeleteOutlinedIcon aria-hidden /></IconButton></Tooltip>
        </div>
        <div className="sm:hidden">
          <ActionMenu
            actions={[{ accessibleLabel: `${t("files.config.deleteRuleSet")} ${accessibleLabel}`, icon: <DeleteOutlinedIcon aria-hidden className="mr-2" fontSize="small" />, label: t("actions.delete"), onSelect: onDelete, tone: "danger" }]}
            buttonSize="small"
            label={t("resourceList.moreActions", { title: `${t("files.config.ruleSet")} ${accessibleLabel}` })}
          />
        </div>
      </DenseConfigRow>
      <Collapse in={open} timeout="auto" unmountOnExit>
        <div className={configEditorPanelClassName} id={contentID}>
          <div className={ruleSetHeaderClassName(presentation.headerLayout)}>
            <TextField fullWidth required label={t("labels.name")} size="small" value={ruleSet.name} onChange={(event) => onUpdate({ name: event.target.value })} />
            <ui.RuleSetFields
              behaviorOptions={behaviorOptions}
              draft={ruleSet}
              onUpdate={onUpdate}
            />
            {presentation.sourceMode === "switchable" ? <ToggleButtonGroup exclusive aria-label={t("files.config.source")} className="w-fit" size="small" value={ruleSet.source} onChange={(_event, value: RuleSetDraft["source"] | null) => { if (value) onUpdate({ source: value }); }}><ToggleButton value="inline">{t("files.config.ruleSetSourceInline")}</ToggleButton><ToggleButton value="remote">{t("files.config.ruleSetSourceRemote")}</ToggleButton></ToggleButtonGroup> : null}
          </div>
          {remote ? (
            <><TextField error={Boolean(ruleSet.url) && !isHTTPURL(ruleSet.url)} fullWidth required helperText={Boolean(ruleSet.url) && !isHTTPURL(ruleSet.url) ? t("files.config.invalidHttpUrl") : undefined} label={t("files.config.url")} size="small" slotProps={{ htmlInput: { pattern: "https?://.+" } }} type="url" value={ruleSet.url} onChange={(event) => onUpdate({ url: event.target.value })} />{presentation.remoteFields === "format-interval" ? <div className="grid gap-3 sm:grid-cols-[minmax(0,1fr)_11rem]"><SelectField label={t("files.config.format")} options={[...adapter.ruleSets.formatOptions]} size="small" value={ruleSet.format} onChange={(value) => onUpdate(adapter.ruleSets.formatPatch(ruleSet.url, value))} /><TextField fullWidth required label={t("files.config.updateInterval")} size="small" type={presentation.intervalInputType} value={ruleSet.interval} onChange={(event) => onUpdate({ interval: event.target.value })} /></div> : null}</>
          ) : <TextField fullWidth multiline required label={t("files.form.content")} minRows={2} size="small" value={ruleSet.payloadText} onChange={(event) => onUpdate({ payloadText: event.target.value })} />}
        </div>
      </Collapse>
    </div>
  );
}

export function RuleListEditor({ adapter, defaultExpanded = false, groups, issues, namingLocale = "en-US", nodes, onChange, rules, ruleSets, ui }: {
  adapter: StructuredFileConfigurationAdapter;
  defaultExpanded?: boolean;
  groups: GroupDraft[];
  issues: ConfigValidationIssue[];
  namingLocale?: ConfigNamingLocale;
  nodes: ConfigNodeSummary[];
  onChange: (value: RuleDraft[]) => void;
  rules: RuleDraft[];
  ruleSets: RuleSetDraft[];
  ui: Readonly<StructuredConfigurationFieldSlots>;
}) {
  const { t } = useI18n();
  const [openRuleID, setOpenRuleID] = useState<string | null>(null);
  const options = adapter.rules.typeOptions(t);
  const policyOptions = policyReferenceOptions(adapter.references, nodes, groups);
  const ruleSetNames = ruleSetReferenceOptions(ruleSets);
  const sensors = useSortableSensors();
  const update = (index: number, patch: Partial<RuleDraft>) => onChange(replaceAt(rules, index, { ...rules[index], ...patch }));
  function moveRule(index: number, direction: -1 | 1) {
    const nextIndex = index + direction;
    if (nextIndex < 0 || nextIndex >= rules.length) return;
    onChange(arrayMove(rules, index, nextIndex));
    setOpenRuleID(null);
  }
  function handleDragEnd(event: DragEndEvent) {
    if (!event.over || event.active.id === event.over.id) return;
    const from = rules.findIndex((rule) => rule.id === event.active.id);
    const to = rules.findIndex((rule) => rule.id === event.over?.id);
    if (from < 0 || to < 0) return;
    onChange(arrayMove(rules, from, to));
    setOpenRuleID(null);
  }
  return (
    <WorkbenchGroupSection count={rules.length} defaultExpanded={defaultExpanded} headerActions={<Button aria-label={t("files.config.addRule")} startIcon={<AddIcon aria-hidden />} type="button" variant="outlined" onClick={() => onChange([...rules, adapter.rules.create(rules.length, namingLocale)])}>{t("actions.add")}</Button>} id="config-routing-rules" label={t("files.config.rules")} severity={severityForIssues(issues)} severityLabel={issues.length ? t("files.config.statusNeedsAttention") : t("files.config.statusValid")} summary={issueSummary(issues, t, t("files.config.orderedRules"))}>
      <SectionIssues issues={issues} />
      <div className="overflow-hidden rounded-md border border-divider">
        <DndContext collisionDetection={closestCenter} sensors={sensors} onDragEnd={handleDragEnd}>
          <SortableContext items={rules.map((rule) => rule.id)} strategy={verticalListSortingStrategy}>
            {rules.map((rule, index) => (
              <SortableRuleRow
                id={rule.id}
                index={index}
                key={rule.id}
                adapter={adapter}
                open={openRuleID === rule.id}
                options={options}
                policyOptions={policyOptions}
                rule={rule}
                ruleSetNames={ruleSetNames}
                total={rules.length}
                ui={ui}
                onDelete={() => {
                  onChange(removeAt(rules, index));
                  setOpenRuleID(null);
                }}
                onMove={(direction) => moveRule(index, direction)}
                onOpenChange={() => setOpenRuleID((current) => current === rule.id ? null : rule.id)}
                onUpdate={(patch) => update(index, patch)}
              />
            ))}
          </SortableContext>
        </DndContext>
      </div>
    </WorkbenchGroupSection>
  );
}

function SortableRuleRow({ adapter, id, index, onDelete, onMove, onOpenChange, onUpdate, open, options, policyOptions, rule, ruleSetNames, total, ui }: {
  adapter: StructuredFileConfigurationAdapter;
  id: string;
  index: number;
  onDelete: () => void;
  onMove: (direction: -1 | 1) => void;
  onOpenChange: () => void;
  onUpdate: (patch: Partial<RuleDraft>) => void;
  open: boolean;
  options: { label: string; value: string }[];
  policyOptions: ReturnType<typeof policyReferenceOptions>;
  rule: RuleDraft;
  ruleSetNames: string[];
  total: number;
  ui: Readonly<StructuredConfigurationFieldSlots>;
}) {
  const { t } = useI18n();
  const { attributes, listeners, setActivatorNodeRef, setNodeRef, transform, transition } = useSortable({ id });
  const hasValue = adapter.rules.requiresValue(rule.type);
  const referencesRuleSet = adapter.rules.referencesRuleSet(rule.type);
  const currentRuleSetNames = referencesRuleSet && rule.value && !ruleSetNames.includes(rule.value) ? [...ruleSetNames, rule.value] : ruleSetNames;
  const contentID = `rule-${id}-editor`;
  return (
    <div aria-label={`${t("files.config.rule")} ${index + 1}`} className="border-t border-divider bg-background-paper first:border-t-0" ref={setNodeRef} role="group" style={{ transform: CSS.Transform.toString(transform), transition }}>
      <DenseConfigRow>
        <div className="flex items-center gap-0.5"><Tooltip title={`${t("files.config.dragRule")} ${index + 1}`}><IconButton {...attributes} {...listeners} aria-label={`${t("files.config.dragRule")} ${index + 1}`} className="hidden sm:inline-flex" ref={setActivatorNodeRef} size="small" style={{ touchAction: "none" }} type="button"><DragIndicatorIcon aria-hidden fontSize="small" /></IconButton></Tooltip><Typography className="max-sm:inline" color="text.secondary" component="span" variant="caption">{index + 1}</Typography></div>
        <IconButton aria-controls={open ? contentID : undefined} aria-expanded={open} aria-label={`${open ? t("files.config.collapseRule") : t("files.config.expandRule")} ${index + 1}`} size="small" type="button" onClick={onOpenChange}>
          {open ? <KeyboardArrowDownIcon aria-hidden fontSize="small" /> : <KeyboardArrowRightIcon aria-hidden fontSize="small" />}
        </IconButton>
        <ConfigRowSummary
          primary={<><span>{rule.type.toUpperCase()}</span>{rule.value ? <span> {rule.value}</span> : null}<span> → </span><span>{rule.policy || t("files.config.policy")}</span></>}
          secondary={rule.noResolve ? ["no-resolve"] : []}
        />
        <RowOrderActions deleteLabel={`${t("files.config.deleteRule")} ${index + 1}`} downLabel={`${t("files.config.moveRuleDown")} ${index + 1}`} downDisabled={index === total - 1} mobileMenuLabel={t("resourceList.moreActions", { title: `${t("files.config.rule")} ${index + 1}` })} upLabel={`${t("files.config.moveRuleUp")} ${index + 1}`} upDisabled={index === 0} onDelete={onDelete} onDown={() => onMove(1)} onUp={() => onMove(-1)} />
      </DenseConfigRow>
      <Collapse in={open} timeout="auto" unmountOnExit>
        <div className={configEditorPanelClassName} id={contentID}>
          <div className={hasValue ? "grid gap-3 md:grid-cols-[minmax(8rem,0.55fr)_minmax(0,1fr)_minmax(0,1fr)_auto] md:items-center" : "grid gap-3 md:grid-cols-[minmax(8rem,0.55fr)_minmax(0,1fr)_minmax(0,1fr)] md:items-center"}>
            <SelectField label={t("files.config.behavior")} options={options} size="small" value={rule.type} onChange={(value) => onUpdate(adapter.rules.transitionType(rule, value))} />
            {hasValue ? referencesRuleSet ? (
              <SelectField label={t("files.config.ruleValue")} options={currentRuleSetNames.map((value) => ({ label: value, value }))} size="small" value={rule.value} onChange={(value) => onUpdate({ value })} />
            ) : (
              <TextField fullWidth required label={t("files.config.ruleValue")} size="small" value={rule.value} onChange={(event) => onUpdate({ value: event.target.value })} />
            ) : <TextField disabled fullWidth label={t("files.config.ruleValue")} placeholder={t("files.config.ruleValueNotRequired")} size="small" value="" />}
            <CreatableReferenceField label={t("files.config.policy")} options={policyOptions} value={rule.policy} onChange={(policy) => onUpdate({ policy })} />
            <ui.RuleFields
              draft={rule}
              supportsNoResolve={adapter.rules.supportsNoResolve(rule.type)}
              onUpdate={onUpdate}
            />
          </div>
        </div>
      </Collapse>
    </div>
  );
}

function ruleSetHeaderClassName(layout: RuleSetHeaderLayout): string {
  switch (layout) {
    case "name-fields-source":
      return "grid gap-3 md:grid-cols-[minmax(0,1fr)_minmax(8rem,0.35fr)_auto] md:items-center";
    case "name-fields":
      return "grid gap-3 md:grid-cols-2 md:items-center";
    case "name-source":
      return "grid gap-3 md:grid-cols-[minmax(0,1fr)_auto] md:items-center";
    case "name":
      return "grid gap-3";
  }
}

function isHTTPURL(value: string): boolean { try { const url = new URL(value); return url.protocol === "http:" || url.protocol === "https:"; } catch { return false; } }
