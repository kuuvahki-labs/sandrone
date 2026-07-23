import { closestCenter, DndContext, type DragEndEvent } from "@dnd-kit/core";
import { arrayMove, SortableContext, useSortable, verticalListSortingStrategy } from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import AddIcon from "@mui/icons-material/Add";
import DragIndicatorIcon from "@mui/icons-material/DragIndicator";
import Autocomplete from "@mui/material/Autocomplete";
import Button from "@mui/material/Button";
import IconButton from "@mui/material/IconButton";
import TextField from "@mui/material/TextField";
import Typography from "@mui/material/Typography";

import type { ConfigReferenceOption } from "~/features/files/config/model/references";
import { useI18n } from "~/shared/i18n/context";

import { removeAt, replaceAt, RowOrderActions, useSortableSensors } from "./editor-shared";

export function OrderedReferenceList({ label, onChange, options, values }: {
  label: string;
  onChange: (values: string[]) => void;
  options: ConfigReferenceOption[];
  values: string[];
}) {
  const { t } = useI18n();
  const sensors = useSortableSensors();
  const ids = values.map((_value, index) => `reference-${index}`);
  function handleDragEnd(event: DragEndEvent) {
    if (!event.over || event.active.id === event.over.id) return;
    const from = ids.indexOf(String(event.active.id));
    const to = ids.indexOf(String(event.over.id));
    if (from >= 0 && to >= 0) onChange(arrayMove(values, from, to));
  }
  return (
    <div className="grid gap-2">
      <DndContext collisionDetection={closestCenter} sensors={sensors} onDragEnd={handleDragEnd}>
        <SortableContext items={ids} strategy={verticalListSortingStrategy}>
          {values.map((value, index) => (
            <SortableReferenceRow
              id={ids[index]}
              index={index}
              key={ids[index]}
              label={label}
              options={options}
              total={values.length}
              value={value}
              onChange={(next) => onChange(replaceAt(values, index, next))}
              onDelete={() => onChange(removeAt(values, index))}
              onMove={(direction) => onChange(arrayMove(values, index, index + direction))}
            />
          ))}
        </SortableContext>
      </DndContext>
      <div className="flex justify-end">
        <Button startIcon={<AddIcon aria-hidden />} type="button" variant="outlined" onClick={() => onChange([...values, ""])}>
          {t("files.config.referenceAdd", { label })}
        </Button>
      </div>
    </div>
  );
}

function SortableReferenceRow({ id, index, label, onChange, onDelete, onMove, options, total, value }: {
  id: string;
  index: number;
  label: string;
  onChange: (value: string) => void;
  onDelete: () => void;
  onMove: (direction: -1 | 1) => void;
  options: ConfigReferenceOption[];
  total: number;
  value: string;
}) {
  const { t } = useI18n();
  const { attributes, listeners, setActivatorNodeRef, setNodeRef, transform, transition } = useSortable({ id });
  return (
    <div className="grid min-w-0 grid-cols-[auto_minmax(0,1fr)_auto] items-center gap-1" ref={setNodeRef} style={{ transform: CSS.Transform.toString(transform), transition }}>
      <IconButton
        {...attributes}
        {...listeners}
        aria-label={t("files.config.referenceDrag", { label, index: index + 1 })}
        ref={setActivatorNodeRef}
        size="small"
        style={{ touchAction: "none" }}
        type="button"
      >
        <DragIndicatorIcon aria-hidden fontSize="small" />
      </IconButton>
      <CreatableReferenceField
        label={t("files.config.referenceWithIndex", { label, index: index + 1 })}
        options={options}
        value={value}
        onChange={onChange}
      />
      <RowOrderActions
        deleteLabel={t("files.config.referenceDelete", { label, index: index + 1 })}
        downDisabled={index === total - 1}
        downLabel={t("files.config.referenceMoveDown", { label, index: index + 1 })}
        upDisabled={index === 0}
        upLabel={t("files.config.referenceMoveUp", { label, index: index + 1 })}
        onDelete={onDelete}
        onDown={() => onMove(1)}
        onUp={() => onMove(-1)}
      />
    </div>
  );
}

export function CreatableReferenceField({ label, onChange, options, value }: {
  label: string;
  onChange: (value: string) => void;
  options: ConfigReferenceOption[];
  value: string;
}) {
  const selected = options.find((option) => option.value === value) ?? value;
  return (
    <Autocomplete<ConfigReferenceOption, false, false, true>
      autoHighlight
      autoSelect
      freeSolo
      options={options}
      value={selected}
      getOptionLabel={(option) => typeof option === "string" ? option : option.value}
      isOptionEqualToValue={(option, current) => typeof current !== "string" && option.value === current.value}
      renderInput={(params) => <TextField {...params} fullWidth label={label} size="small" />}
      renderOption={(props, option) => (
        <li {...props} key={`${option.kind}:${option.value}`}>
          <span className="grid min-w-0 gap-0.5">
            <Typography className="break-words" component="span" variant="body2">{option.value}</Typography>
            {option.detail ? <Typography className="break-words" color="text.secondary" component="span" variant="caption">{option.detail}</Typography> : null}
          </span>
        </li>
      )}
      onChange={(_event, next) => onChange(typeof next === "string" ? next : next?.value ?? "")}
    />
  );
}
