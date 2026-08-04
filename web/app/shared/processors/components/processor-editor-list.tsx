import { useEffect, useMemo, useState } from "react";
import type { ReactNode } from "react";
import AddIcon from "@mui/icons-material/Add";
import ArrowDownwardIcon from "@mui/icons-material/ArrowDownward";
import ArrowUpwardIcon from "@mui/icons-material/ArrowUpward";
import DeleteOutlinedIcon from "@mui/icons-material/DeleteOutlined";
import EditOutlinedIcon from "@mui/icons-material/EditOutlined";
import Button from "@mui/material/Button";
import Collapse from "@mui/material/Collapse";
import Divider from "@mui/material/Divider";
import IconButton from "@mui/material/IconButton";
import Paper from "@mui/material/Paper";
import TextField from "@mui/material/TextField";
import Tooltip from "@mui/material/Tooltip";
import Typography from "@mui/material/Typography";

import { type Translator, useI18n } from "~/shared/i18n/context";
import {
  cleanParams,
  customProcessorName,
  keyValueTextToReplacementPatch,
  objectToKeyValueText,
  type ProcessorDraft,
  stringValue,
} from "~/shared/processors/model";
import type { ProcessorDetail } from "~/shared/resources/types";
import { HighlightedTextarea } from "~/shared/ui/code-editor";
import {
  SelectField,
  type SelectOption,
} from "~/shared/ui/form-fields";

export type ProcessorParamsEditorProps = {
  draft: ProcessorDraft;
  onChange: (patch: Record<string, unknown>) => void;
};

export function KeyValueParamsEditor({
  className,
  label,
  onChange,
  params,
  placeholder = "key=value",
}: {
  className?: string;
  label?: string;
  onChange: (patch: Record<string, unknown>) => void;
  params: Record<string, unknown>;
  placeholder?: string;
}) {
  const { t } = useI18n();
  const [text, setText] = useState(() => objectToKeyValueText(params));

  return (
    <HighlightedTextarea
      className={className}
      label={label ?? t("processor.paramsKeyValue")}
      language="text"
      minRows={4}
      placeholder={placeholder}
      value={text}
      onChange={(event) => {
        const nextText = event.target.value;
        setText(nextText);
        onChange(keyValueTextToReplacementPatch(params, nextText));
      }}
    />
  );
}

type ProcessorEditorListProps = {
  addProcessorDrafts?: (type: string, current: ProcessorDraft[]) => ProcessorDraft[];
  createDraftId: (index?: number) => string;
  defaultParams: (type: string) => Record<string, unknown>;
  defaultType: string;
  defaultValue?: ProcessorDetail[];
  draftProcessors?: (processors: ProcessorDetail[]) => ProcessorDraft[];
  paramsEditor: (props: ProcessorParamsEditorProps) => ReactNode;
  processorOptions: SelectOption[];
  serializeDraft: (draft: ProcessorDraft) => ProcessorDetail;
  onDirty?: () => void;
  onValueChange?: (processors: ProcessorDetail[]) => void;
};

export function ProcessorEditorList({
  addProcessorDrafts,
  createDraftId,
  defaultParams,
  defaultType,
  defaultValue = [],
  draftProcessors,
  paramsEditor,
  processorOptions,
  serializeDraft,
  onDirty,
  onValueChange,
}: ProcessorEditorListProps) {
  const { t } = useI18n();
  const [drafts, setDrafts] = useState(() => draftProcessors ? draftProcessors(defaultValue) : defaultValue.map((processor, index) => draftFromProcessor(processor, index, createDraftId)));
  const [newType, setNewType] = useState(defaultType);
  const [editingIds, setEditingIds] = useState<Set<string>>(() => new Set());
  const processors = useMemo(() => drafts.map(serializeDraft), [drafts, serializeDraft]);
  const serialized = useMemo(() => JSON.stringify(processors), [processors]);

  useEffect(() => onValueChange?.(processors), [onValueChange, processors]);

  function labelForType(type: string): string {
    return processorLabel(type, processorOptions);
  }

  function updateDraft(index: number, patch: Partial<ProcessorDraft>) {
    commitDrafts(drafts.map((draft, i) => (i === index ? { ...draft, ...patch } : draft)));
  }

  function updateParams(index: number, patch: Record<string, unknown>) {
    commitDrafts(drafts.map((draft, i) => (i === index ? { ...draft, params: cleanParams({ ...draft.params, ...patch }) } : draft)));
  }

  function addProcessor() {
    commitDrafts(addProcessorDrafts ? addProcessorDrafts(newType, drafts) : [...drafts, {
      id: createDraftId(),
      name: "",
      type: newType,
      params: defaultParams(newType),
    }]);
  }

  function toggleEditor(id: string) {
    setEditingIds((current) => {
      const next = new Set(current);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  }

  function moveProcessor(index: number, direction: -1 | 1) {
    const next = [...drafts];
    const target = index + direction;
    if (target < 0 || target >= next.length) return;
    [next[index], next[target]] = [next[target], next[index]];
    commitDrafts(next);
  }

  function removeProcessor(id: string) {
    commitDrafts(drafts.filter((draft) => draft.id !== id));
    setEditingIds((current) => {
      const next = new Set(current);
      next.delete(id);
      return next;
    });
  }

  function commitDrafts(next: ProcessorDraft[]) {
    if (JSON.stringify(next.map(serializeDraft)) === serialized) return;
    setDrafts(next);
    onDirty?.();
  }

  return (
    <div className="grid gap-4">
      <input name="processors" type="hidden" value={serialized} />
      {drafts.length ? (
        <div className="grid gap-4">
          {drafts.map((draft, index) => {
            const typeLabel = labelForType(draft.type);
            const displayName = processorDisplayName(draft, index, labelForType, t);
            const isEditing = editingIds.has(draft.id);
            const editorId = `${draft.id}-name-editor`;
            const groupLabel = customProcessorName(draft, labelForType) ? displayName : typeLabel;
            return (
              <Paper aria-label={t("processor.group", { label: groupLabel })} className="p-4" component="section" key={draft.id} role="group" variant="outlined">
                <div className="grid gap-4">
                  <div className="flex items-center justify-between gap-4">
                    <div className="min-w-0">
                      <Typography className="break-words" component="h4" variant="subtitle1">
                        {displayName}
                      </Typography>
                      <Typography className="break-words" color="text.secondary" variant="body2">
                        {typeLabel}
                      </Typography>
                    </div>
                    <div className="flex gap-1">
                      <Tooltip title={isEditing ? t("processor.collapseNameEdit") : t("processor.editName")}>
                        <IconButton aria-controls={editorId} aria-expanded={isEditing} aria-label={isEditing ? t("processor.collapseNameEdit") : t("processor.editName")} size="small" type="button" onClick={() => toggleEditor(draft.id)}>
                          <EditOutlinedIcon aria-hidden fontSize="small" />
                        </IconButton>
                      </Tooltip>
                      <IconButton aria-label={t("processor.moveUp")} disabled={index === 0} size="small" type="button" onClick={() => moveProcessor(index, -1)}>
                        <ArrowUpwardIcon aria-hidden fontSize="small" />
                      </IconButton>
                      <IconButton aria-label={t("processor.moveDown")} disabled={index === drafts.length - 1} size="small" type="button" onClick={() => moveProcessor(index, 1)}>
                        <ArrowDownwardIcon aria-hidden fontSize="small" />
                      </IconButton>
                      <IconButton aria-label={t("processor.delete")} color="error" size="small" type="button" onClick={() => removeProcessor(draft.id)}>
                        <DeleteOutlinedIcon aria-hidden fontSize="small" />
                      </IconButton>
                    </div>
                  </div>
                  <Collapse id={editorId} in={isEditing} timeout="auto" unmountOnExit>
                    <div className="pt-2">
                      <TextField fullWidth label={t("labels.name")} placeholder={t("processor.namePlaceholder")} value={draft.name} onChange={(event) => updateDraft(index, { name: event.target.value })} />
                    </div>
                  </Collapse>
                  <Divider />
                  <div className="grid gap-4 md:grid-cols-2">
                    {paramsEditor({ draft, onChange: (patch) => updateParams(index, patch) })}
                  </div>
                </div>
              </Paper>
            );
          })}
        </div>
      ) : null}

      <Paper className="p-4" variant="outlined">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
          <SelectField label={t("processor.type")} options={processorOptions} value={newType} onChange={setNewType} />
          <Button aria-label={t("processor.add")} className="shrink-0 whitespace-nowrap" startIcon={<AddIcon aria-hidden fontSize="small" />} type="button" variant="contained" onClick={addProcessor}>
            {t("actions.add")}
          </Button>
        </div>
      </Paper>
    </div>
  );
}

function draftFromProcessor(processor: ProcessorDetail, index: number, createDraftId: (index?: number) => string): ProcessorDraft {
  return { id: createDraftId(index), name: stringValue(processor.name), type: processor.type || "script", params: cleanParams(processor.params ?? {}) };
}

function processorDisplayName(draft: ProcessorDraft, index: number, labelForType: (type: string) => string, t: Translator): string {
  return customProcessorName(draft, labelForType) || t("processor.defaultName", { index: index + 1 });
}

function processorLabel(type: string, options: SelectOption[]): string {
  return options.find((option) => option.value === type)?.label ?? type;
}
