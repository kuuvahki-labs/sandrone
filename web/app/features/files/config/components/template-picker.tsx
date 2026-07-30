import { type ReactNode, useId, useState } from "react";
import Alert from "@mui/material/Alert";
import Button from "@mui/material/Button";
import Dialog from "@mui/material/Dialog";
import DialogActions from "@mui/material/DialogActions";
import DialogContent from "@mui/material/DialogContent";
import DialogTitle from "@mui/material/DialogTitle";
import FormControl from "@mui/material/FormControl";
import FormLabel from "@mui/material/FormLabel";
import Paper from "@mui/material/Paper";
import Radio from "@mui/material/Radio";
import RadioGroup from "@mui/material/RadioGroup";
import Typography from "@mui/material/Typography";

export interface ConfigTemplateChoice {
  description: string;
  groupCount: number;
  id: string;
  name: string;
  ruleCount: number;
  ruleSetCount: number;
}

export interface ConfigTemplatePickerCopy {
  cancel: string;
  confirm: string;
  confirmAccessibleLabel: string;
  dialogDescription: string;
  dialogTitle: string;
  groups: (count: number) => string;
  label: string;
  ruleSets: (count: number) => string;
  rules: (count: number) => string;
}

export interface ConfigTemplatePickerProps {
  choices: readonly ConfigTemplateChoice[];
  confirmBeforeApply?: boolean;
  copy?: Partial<ConfigTemplatePickerCopy>;
  currentTemplateId?: string;
  labelledBy?: string;
  onRequestApply: (choice: ConfigTemplateChoice) => void;
}

export interface ConfigTemplateApplyDialogProps {
  choice: ConfigTemplateChoice;
  copy?: Partial<ConfigTemplatePickerCopy>;
  open: boolean;
  onCancel: () => void;
  onConfirm: (choice: ConfigTemplateChoice) => void;
}

export interface ConfigTemplateAppliedNoticeProps {
  message: ReactNode;
  onUndo?: () => void;
  undoLabel?: string;
}

const defaultCopy: ConfigTemplatePickerCopy = {
  cancel: "Cancel",
  confirm: "Replace",
  confirmAccessibleLabel: "Replace configuration",
  dialogDescription: "This replaces the current groups, rule sets, and rules with:",
  dialogTitle: "Replace configuration?",
  groups: (count) => `${count} ${count === 1 ? "group" : "groups"}`,
  label: "Configuration template",
  ruleSets: (count) => `${count} ${count === 1 ? "rule set" : "rule sets"}`,
  rules: (count) => `${count} ${count === 1 ? "rule" : "rules"}`,
};

export function ConfigTemplatePicker({
  choices,
  confirmBeforeApply = false,
  copy,
  currentTemplateId,
  labelledBy,
  onRequestApply,
}: ConfigTemplatePickerProps) {
  const [pendingChoice, setPendingChoice] = useState<ConfigTemplateChoice | null>(null);
  const labelId = useId();
  const choiceIdPrefix = useId();
  const resolvedCopy = { ...defaultCopy, ...copy };

  function requestSelection(choiceId: string) {
    const choice = choices.find((entry) => entry.id === choiceId);
    if (!choice) return;

    if (confirmBeforeApply) {
      setPendingChoice(choice);
      return;
    }

    setPendingChoice(null);
    onRequestApply(choice);
  }

  function confirmSelection(choice: ConfigTemplateChoice) {
    setPendingChoice(null);
    onRequestApply(choice);
  }

  return (
    <>
      <FormControl className="min-w-0" component="fieldset" fullWidth>
        {!labelledBy ? <FormLabel className="min-w-0" id={labelId}>
          {resolvedCopy.label}
        </FormLabel> : null}
        <RadioGroup
          aria-labelledby={labelledBy ?? labelId}
          className={`${labelledBy ? "" : "mt-2 "}grid min-w-0 grid-cols-1 gap-2 sm:grid-cols-3`}
          value={currentTemplateId ?? ""}
          onChange={(event, value) => {
            if (confirmBeforeApply) event.stopPropagation();
            requestSelection(value);
          }}
        >
          {choices.map((choice) => {
            const descriptionId = `${choiceIdPrefix}-${choice.id}-description`;
            const countsId = `${choiceIdPrefix}-${choice.id}-counts`;
            const selected = choice.id === currentTemplateId;
            return (
              <Paper
                className={`flex min-h-11 min-w-0 cursor-pointer items-start gap-1 rounded-md border p-2 ${selected ? "border-primary bg-action-selected" : "border-divider"}`}
                component="label"
                data-selected={selected ? "true" : "false"}
                key={choice.id}
                variant="outlined"
              >
                <Radio
                  className="shrink-0 p-2"
                  size="small"
                  slotProps={{ input: { "aria-describedby": `${descriptionId} ${countsId}`, "aria-label": choice.name } }}
                  value={choice.id}
                />
                <span className="grid min-w-0 flex-1 gap-0.5 py-1">
                  <Typography className="min-w-0 truncate font-semibold" component="span" variant="body2">
                    {choice.name}
                  </Typography>
                  <Typography className="min-w-0 break-words" color="text.secondary" component="span" id={descriptionId} variant="caption">
                    {choice.description}
                  </Typography>
                  <TemplateCounts choice={choice} copy={resolvedCopy} id={countsId} />
                </span>
              </Paper>
            );
          })}
        </RadioGroup>
      </FormControl>
      {pendingChoice ? (
        <ConfigTemplateApplyDialog
          choice={pendingChoice}
          copy={resolvedCopy}
          open
          onCancel={() => setPendingChoice(null)}
          onConfirm={confirmSelection}
        />
      ) : null}
    </>
  );
}

export function ConfigTemplateApplyDialog({ choice, copy, onCancel, onConfirm, open }: ConfigTemplateApplyDialogProps) {
  const titleId = useId();
  const descriptionId = useId();
  const resolvedCopy = { ...defaultCopy, ...copy };

  return (
    <Dialog
      fullWidth
      open={open}
      aria-describedby={descriptionId}
      aria-labelledby={titleId}
      maxWidth="xs"
      onClose={onCancel}
    >
      <DialogTitle id={titleId}>{resolvedCopy.dialogTitle}</DialogTitle>
      <DialogContent className="grid min-w-0 gap-3">
        <Typography className="font-semibold" component="p" variant="body1">{choice.name}</Typography>
        <Typography color="text.secondary" id={descriptionId} variant="body2">{resolvedCopy.dialogDescription}</Typography>
        <TemplateCounts choice={choice} copy={resolvedCopy} />
      </DialogContent>
      <DialogActions className="flex-wrap px-3 pb-3">
        <Button className="min-h-11" type="button" onClick={onCancel}>{resolvedCopy.cancel}</Button>
        <Button aria-label={resolvedCopy.confirmAccessibleLabel} className="min-h-11 whitespace-normal" type="button" variant="contained" onClick={() => onConfirm(choice)}>{resolvedCopy.confirm}</Button>
      </DialogActions>
    </Dialog>
  );
}

export function ConfigTemplateAppliedNotice({ message, onUndo, undoLabel = "Undo" }: ConfigTemplateAppliedNoticeProps) {
  return (
    <Alert
      action={onUndo ? (
        <Button className="min-h-11 shrink-0 whitespace-normal" color="inherit" size="small" type="button" onClick={onUndo}>
          {undoLabel}
        </Button>
      ) : undefined}
      className="min-w-0 items-center py-0"
      role="status"
      severity="success"
      variant="outlined"
    >
      <Typography className="break-words" component="span" variant="body2">{message}</Typography>
    </Alert>
  );
}

function TemplateCounts({ choice, copy, id }: { choice: ConfigTemplateChoice; copy: ConfigTemplatePickerCopy; id?: string }) {
  const metrics = [
    { key: "groups", value: copy.groups(choice.groupCount) },
    { key: "rule-sets", value: copy.ruleSets(choice.ruleSetCount) },
    { key: "rules", value: copy.rules(choice.ruleCount) },
  ];

  return (
    <>
      {id ? <span className="sr-only" id={id}>{metrics.map((metric) => metric.value).join(" · ")}</span> : null}
      <span aria-hidden={id ? true : undefined} className="flex min-w-0 flex-wrap items-center gap-x-1.5 gap-y-0.5 text-text-secondary">
        {metrics.map((metric, index) => (
          <span className="inline-flex items-center gap-1" key={metric.key}>
            {index > 0 ? <span>·</span> : null}
            <Typography className="whitespace-nowrap" component="span" variant="caption">
              {metric.value}
            </Typography>
          </span>
        ))}
      </span>
    </>
  );
}
