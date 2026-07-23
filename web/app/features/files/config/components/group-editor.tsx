import { useEffect, useRef, useState } from "react";
import { closestCenter, DndContext, type DragEndEvent } from "@dnd-kit/core";
import { arrayMove, SortableContext, useSortable, verticalListSortingStrategy } from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import AddIcon from "@mui/icons-material/Add";
import DragIndicatorIcon from "@mui/icons-material/DragIndicator";
import KeyboardArrowDownIcon from "@mui/icons-material/KeyboardArrowDown";
import KeyboardArrowRightIcon from "@mui/icons-material/KeyboardArrowRight";
import Button from "@mui/material/Button";
import Checkbox from "@mui/material/Checkbox";
import Collapse from "@mui/material/Collapse";
import FormControlLabel from "@mui/material/FormControlLabel";
import IconButton from "@mui/material/IconButton";
import TextField from "@mui/material/TextField";
import Tooltip from "@mui/material/Tooltip";

import {
  applyGroupFieldsPatch,
  type GroupDraft,
  toGroupFieldsDraft,
} from "~/features/files/config/model/editor-model";
import type { ConfigNamingLocale } from "~/features/files/config/model/naming";
import type { ConfigNodeSummary } from "~/features/files/config/model/node-source";
import { memberReferenceOptions } from "~/features/files/config/model/references";
import type { ConfigValidationIssue } from "~/features/files/config/model/relations";
import type { StructuredFileConfigurationAdapter } from "~/features/files/drivers/core/file-driver";
import type { StructuredConfigurationFieldSlots } from "~/features/files/editor/file-driver-ui";
import { useI18n } from "~/shared/i18n/context";
import { SelectField } from "~/shared/ui/form-fields";

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
import { OrderedReferenceList } from "./reference-fields";

export function ProxyGroupEditor({ adapter, defaultExpanded = true, groups, inboundReferences, issues, namingLocale = "en-US", nodes, onChange, ui }: {
  adapter: StructuredFileConfigurationAdapter;
  defaultExpanded?: boolean;
  groups: GroupDraft[];
  inboundReferences: Record<string, number>;
  issues: ConfigValidationIssue[];
  namingLocale?: ConfigNamingLocale;
  nodes: ConfigNodeSummary[];
  onChange: (value: GroupDraft[]) => void;
  ui: Readonly<StructuredConfigurationFieldSlots>;
}) {
  const { t } = useI18n();
  const [openGroup, setOpenGroup] = useState<number | null>(null);
  const sensors = useSortableSensors();
  const groupIDCounter = useRef(groups.length);
  const [sortableIDs, setSortableIDs] = useState(() => groups.map((_group, index) => `proxy-group-${index}`));

  function moveGroup(index: number, direction: -1 | 1) {
    const nextIndex = index + direction;
    if (nextIndex < 0 || nextIndex >= groups.length) return;
    setSortableIDs((current) => arrayMove(current, index, nextIndex));
    onChange(arrayMove(groups, index, nextIndex));
    setOpenGroup(null);
  }

  function handleDragEnd(event: DragEndEvent) {
    if (!event.over || event.active.id === event.over.id) return;
    const from = sortableIDs.indexOf(String(event.active.id));
    const to = sortableIDs.indexOf(String(event.over.id));
    if (from < 0 || to < 0) return;
    setSortableIDs((current) => arrayMove(current, from, to));
    onChange(arrayMove(groups, from, to));
    setOpenGroup(null);
  }

  return (
    <WorkbenchGroupSection
      count={groups.length}
      defaultExpanded={defaultExpanded}
      headerActions={<Button aria-label={t("files.config.addGroup")} startIcon={<AddIcon aria-hidden />} type="button" variant="outlined" onClick={() => { setSortableIDs((current) => [...current, `proxy-group-${groupIDCounter.current++}`]); onChange([...groups, adapter.groups.create(namingLocale)]); }}>{t("actions.add")}</Button>}
      id="config-proxy-groups"
      label={t("files.config.proxyGroups")}
      severity={severityForIssues(issues)}
      severityLabel={issues.length ? t("files.config.statusNeedsAttention") : t("files.config.statusValid")}
      summary={issueSummary(issues, t, t("files.config.inboundTotal", { count: Object.values(inboundReferences).reduce((sum, count) => sum + count, 0) }))}
    >
      <SectionIssues issues={issues} />
      <div className="overflow-hidden rounded-md border border-divider">
        <DndContext collisionDetection={closestCenter} sensors={sensors} onDragEnd={handleDragEnd}>
          <SortableContext items={sortableIDs} strategy={verticalListSortingStrategy}>
            {groups.map((group, index) => (
              <SortableProxyGroupRow
                group={group}
                id={sortableIDs[index]}
                adapter={adapter}
                inboundCount={inboundReferences[group.name] ?? 0}
                index={index}
                key={sortableIDs[index]}
                nodes={nodes}
                open={openGroup === index}
                peerGroups={groups}
                total={groups.length}
                ui={ui}
                onDelete={() => { setSortableIDs((current) => removeAt(current, index)); onChange(removeAt(groups, index)); setOpenGroup(null); }}
                onMove={(direction) => moveGroup(index, direction)}
                onOpenChange={() => setOpenGroup((current) => current === index ? null : index)}
                onUpdate={(next) => onChange(replaceAt(groups, index, next))}
              />
            ))}
          </SortableContext>
        </DndContext>
      </div>
    </WorkbenchGroupSection>
  );
}

function SortableProxyGroupRow({ adapter, group, id, inboundCount, index, nodes, onDelete, onMove, onOpenChange, onUpdate, open, peerGroups, total, ui }: {
  adapter: StructuredFileConfigurationAdapter;
  group: GroupDraft; id: string; inboundCount: number; index: number; onDelete: () => void; onMove: (direction: -1 | 1) => void;
  nodes: ConfigNodeSummary[]; onOpenChange: () => void; onUpdate: (value: GroupDraft) => void; open: boolean; peerGroups: GroupDraft[]; total: number;
  ui: Readonly<StructuredConfigurationFieldSlots>;
}) {
  const { t } = useI18n();
  const { attributes, listeners, setActivatorNodeRef, setNodeRef, transform, transition } = useSortable({ id });
  const name = group.name || t("files.config.unnamedGroup");
  const targets = group.members;
  const memberMode = group.memberMode;
  const fixedTargetsRef = useRef(targets.length ? targets : ["$nodes"]);
  const runtimeFilterRef = useRef(group.filter || "(?i)");
  useEffect(() => {
    if (memberMode === "fixed" && targets.length) fixedTargetsRef.current = targets;
  }, [memberMode, targets]);
  const type = group.type;
  const typeLabel = adapter.groups.typeOptions.find((option) => option.value === type)?.label ?? type;
  const memberOptions = memberReferenceOptions(adapter.references, nodes, peerGroups, group.name);
  const patchGroup = (patch: Partial<GroupDraft>) => onUpdate({ ...group, ...patch });

  function updateType(value: string) {
    onUpdate(adapter.groups.transitionType(group, value));
  }

  function updateMemberMode(value: string) {
    const mode = value as GroupDraft["memberMode"];
    if (mode === "runtime-filter" && targets.length) fixedTargetsRef.current = targets;
    if (mode === "fixed" && group.filter) runtimeFilterRef.current = group.filter;
    const next = adapter.groups.transitionMemberMode(group, mode, fixedTargetsRef.current);
    onUpdate(mode === "runtime-filter" ? { ...next, filter: runtimeFilterRef.current } : next);
  }

  return (
    <div className="border-b border-divider bg-background-paper last:border-b-0" ref={setNodeRef} style={{ transform: CSS.Transform.toString(transform), transition }}>
      <DenseConfigRow>
        <Tooltip title={`${t("files.config.dragGroup")} ${name}`}><IconButton {...attributes} {...listeners} aria-label={`${t("files.config.dragGroup")} ${name}`} className="hidden sm:inline-flex" ref={setActivatorNodeRef} size="small" style={{ touchAction: "none" }} type="button"><DragIndicatorIcon aria-hidden fontSize="small" /></IconButton></Tooltip>
        <IconButton aria-label={`${open ? t("files.config.collapseGroup") : t("files.config.expandGroup")} ${name}`} size="small" type="button" onClick={onOpenChange}>{open ? <KeyboardArrowDownIcon aria-hidden fontSize="small" /> : <KeyboardArrowRightIcon aria-hidden fontSize="small" />}</IconButton>
        <ConfigRowSummary
          primary={name}
          secondary={[
            typeLabel,
            ...(memberMode === "runtime-filter"
              ? [t("files.config.groupRuntimeFilterSummary")]
              : targets.length
                ? targets.map((target) => target === "$nodes" ? t("files.config.subscriptionNodes") : target)
                : [t("files.config.noTargets")]),
            t("files.config.inboundCount", { count: inboundCount }),
          ]}
        />
        <RowOrderActions deleteLabel={`${t("files.config.deleteGroup")} ${name}`} downLabel={`${t("files.config.moveGroupDown")} ${name}`} downDisabled={index === total - 1} mobileMenuLabel={t("resourceList.moreActions", { title: name })} upLabel={`${t("files.config.moveGroupUp")} ${name}`} upDisabled={index === 0} onDelete={onDelete} onDown={() => onMove(1)} onUp={() => onMove(-1)} />
      </DenseConfigRow>
      <Collapse in={open} timeout="auto" unmountOnExit>
        <div className={configEditorPanelClassName}>
          <div className="grid gap-3 sm:grid-cols-[minmax(0,1fr)_minmax(12rem,0.55fr)]">
            <TextField fullWidth required label={t("files.config.groupNameWithIndex", { index: index + 1 })} size="small" value={group.name} onChange={(event) => patchGroup({ name: event.target.value })} />
            <SelectField label={t("files.config.groupTypeWithIndex", { index: index + 1 })} options={[...adapter.groups.typeOptions]} size="small" value={type} onChange={updateType} />
          </div>
          <div
            className={adapter.groups.supportsHidden
              ? "grid gap-3 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center"
              : "grid gap-3"}
            data-group-membership-controls=""
          >
            {adapter.groups.supportsRuntimeFilter ? (
              <SelectField
                label={t("files.config.groupMemberSource")}
                options={[
                  { value: "fixed", label: t("files.config.groupMemberSourceFixed") },
                  { value: "runtime-filter", label: t("files.config.groupMemberSourceRuntime") },
                ]}
                size="small"
                value={memberMode}
                onChange={updateMemberMode}
              />
            ) : <span />}
            {adapter.groups.supportsHidden ? (
              <FormControlLabel
                className="m-0 w-fit"
                control={<Checkbox checked={group.hidden === true} size="small" onChange={(event) => patchGroup({ hidden: event.target.checked })} />}
                label={t("files.config.groupHidden")}
              />
            ) : null}
          </div>
          {memberMode === "runtime-filter" ? (
            <div className={adapter.groups.supportsExcludeFilter ? "grid gap-3 sm:grid-cols-2" : "grid gap-3"}>
              <TextField
                error={!adapter.groups.validateFilter(group.filter)}
                fullWidth
                required
                helperText={!adapter.groups.validateFilter(group.filter)
                  ? t("files.config.issueGroupFilterInvalid", { index: index + 1 })
                  : undefined}
                label={t("files.config.groupFilter")}
                size="small"
                value={group.filter}
                onChange={(event) => {
                  runtimeFilterRef.current = event.target.value;
                  patchGroup({ filter: event.target.value });
                }}
              />
              {adapter.groups.supportsExcludeFilter ? <TextField
                error={Boolean(group.excludeFilter) && !adapter.groups.validateFilter(group.excludeFilter)}
                fullWidth
                helperText={Boolean(group.excludeFilter) && !adapter.groups.validateFilter(group.excludeFilter)
                  ? t("files.config.issueGroupFilterInvalid", { index: index + 1 })
                  : undefined}
                label={t("files.config.groupExcludeFilter")}
                size="small"
                value={group.excludeFilter}
                onChange={(event) => patchGroup({ excludeFilter: event.target.value })}
              /> : null}
            </div>
          ) : (
            <OrderedReferenceList
              label={t("files.config.groupMembers")}
              options={memberOptions}
              values={targets}
              onChange={(values) => {
                if (values.length) fixedTargetsRef.current = values;
                patchGroup({ members: values });
              }}
            />
          )}
          <ui.GroupFields
            draft={toGroupFieldsDraft(group)}
            healthCheck={adapter.groups.isHealthCheck(group.type)}
            index={index}
            onUpdate={(patch) => onUpdate(applyGroupFieldsPatch(group, patch))}
          />
        </div>
      </Collapse>
    </div>
  );
}
