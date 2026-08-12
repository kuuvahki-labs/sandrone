import { useCallback, useId } from "react";
import FormControl from "@mui/material/FormControl";
import InputLabel from "@mui/material/InputLabel";
import ListSubheader from "@mui/material/ListSubheader";
import MenuItem from "@mui/material/MenuItem";
import Select from "@mui/material/Select";

export type SelectOption = {
  value: string;
  label: string;
  group?: string;
};

export function SelectField({ className, focusOnMount = false, label, onChange, options, size = "medium", value }: {
  className?: string;
  focusOnMount?: boolean;
  label: string;
  onChange: (value: string) => void;
  options: SelectOption[];
  size?: "medium" | "small";
  value: string;
}) {
  const labelId = useId();
  const selectId = useId();
  const selectedLabel = selectOptionLabel(value, options);
  const focusSelect = useCallback((element: HTMLDivElement | null) => {
    if (focusOnMount) element?.querySelector<HTMLElement>('[role="combobox"]')?.focus();
  }, [focusOnMount]);
  return (
    <FormControl className={className} fullWidth ref={focusSelect} size={size}>
      <InputLabel id={labelId} shrink={value === "" ? true : undefined}>{label}</InputLabel>
      <Select displayEmpty id={selectId} label={label} labelId={labelId} renderValue={() => selectedLabel} value={value} onChange={(event) => onChange(String(event.target.value))}>
        {groupedSelectItems(options)}
      </Select>
    </FormControl>
  );
}

function groupedSelectItems(options: SelectOption[]) {
  let previousGroup = "";
  return options.flatMap((option, index) => {
    const group = option.group?.trim() ?? "";
    const header = group && group !== previousGroup
      ? <ListSubheader key={`group-${group}-${index}`}>{group}</ListSubheader>
      : null;
    previousGroup = group;
    const item = <MenuItem key={option.value || "default"} value={option.value}>{option.label}</MenuItem>;
    return header ? [header, item] : [item];
  });
}

function selectOptionLabel(value: string, options: SelectOption[]): string {
  return options.find((option) => option.value === value)?.label ?? value;
}
