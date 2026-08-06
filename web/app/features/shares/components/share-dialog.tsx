import { type SyntheticEvent, useRef, useState } from "react";
import Alert from "@mui/material/Alert";
import Button from "@mui/material/Button";
import DialogActions from "@mui/material/DialogActions";
import FormControl from "@mui/material/FormControl";
import FormHelperText from "@mui/material/FormHelperText";
import InputLabel from "@mui/material/InputLabel";
import NativeSelect from "@mui/material/NativeSelect";
import Stack from "@mui/material/Stack";
import TextField from "@mui/material/TextField";
import Typography from "@mui/material/Typography";

import type { CopyShareResult } from "~/features/shares/data/create-share-actions";
import { shareCopyFormats } from "~/features/shares/model/share-formats";
import type { ShareItem, ShareTarget } from "~/features/shares/model/types";
import { useI18n } from "~/shared/i18n/context";
import { AppDialog } from "~/shared/ui/dialogs";

import { hasSelectionWithin, selectContents } from "./share-url-selection";

interface ShareDialogProps {
  target: ShareTarget;
  onClose: () => void;
  onCopy: (item: ShareItem) => Promise<CopyShareResult>;
  onSubmit: (form: FormData) => Promise<ShareItem | null>;
}

export function ShareDialog({ target, onClose, onCopy, onSubmit }: ShareDialogProps) {
  const { t } = useI18n();
  const [error, setError] = useState("");
  const [result, setResult] = useState<ShareItem | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const publicUrlElement = useRef<HTMLElement | null>(null);

  async function handleSubmit(event: SyntheticEvent<HTMLFormElement, SubmitEvent>) {
    event.preventDefault();
    setError("");
    setSubmitting(true);
    try {
      const created = await onSubmit(new FormData(event.currentTarget));
      if (created) setResult(created);
    } catch (submitError) {
      setError(submitError instanceof Error ? submitError.message : t("errors.operationFailedRetry"));
    } finally {
      setSubmitting(false);
    }
  }

  async function copyResult() {
    if (result && !(await onCopy(result)).copied) {
      selectContents(publicUrlElement.current);
    }
  }

  if (result) {
    return (
      <AppDialog title={t("messages.shareCreated")} onClose={onClose}>
        <Typography
          className="block cursor-text break-words select-text"
          component="code"
          ref={publicUrlElement}
          onClick={(event) => {
            if (!hasSelectionWithin(event.currentTarget)) {
              selectContents(event.currentTarget);
            }
          }}
        >
          {result.publicUrl}
        </Typography>
        <DialogActions className="px-0 pb-0">
          <Button type="button" onClick={onClose}>{t("share.result.done")}</Button>
          <Button type="button" variant="contained" onClick={() => { void copyResult(); }}>
            {t("shares.actions.copy")}
          </Button>
        </DialogActions>
      </AppDialog>
    );
  }

  return (
    <AppDialog disableClose={submitting} title={t("share.create.title")} onClose={onClose}>
      <Stack component="form" spacing={2} onSubmit={(event) => { void handleSubmit(event); }}>
        <ShareFields target={target} />
        {error ? (
          <Alert severity="error">{error}</Alert>
        ) : null}
        <DialogActions className="px-0 pb-0">
          <Button disabled={submitting} type="button" onClick={onClose}>{t("actions.cancel")}</Button>
          <Button aria-label={t("share.create.save")} disabled={submitting} type="submit" variant="contained">
            {submitting ? t("share.create.saving") : t("actions.save")}
          </Button>
        </DialogActions>
      </Stack>
    </AppDialog>
  );
}

function ShareFields({ target }: { target: ShareTarget }) {
  const { t } = useI18n();
  const [durationPreset, setDurationPreset] = useState<"1d" | "7d" | "long" | "custom">("long");
  const [validUntil, setValidUntil] = useState("");

  function applyDurationPreset(preset: "1d" | "7d" | "long") {
    setDurationPreset(preset);
    if (preset === "long") {
      setValidUntil("");
      return;
    }
    const days = preset === "1d" ? 1 : 7;
    setValidUntil(datetimeLocalValue(new Date(Date.now() + days * 24 * 60 * 60 * 1000)));
  }

  return (
    <>
      <TextField label={t("labels.name")} name="name" placeholder="mobile" defaultValue={target.name} />
      <input name="target_kind" type="hidden" value={target.kind} />
      <input name="target" type="hidden" value={target.name} />
      <TextField disabled label={t("labels.target")} defaultValue={target.name} slotProps={{ htmlInput: { "aria-label": t("share.field.target") } }} />
      {target.kind === "subscription" ? (
        <FormControl fullWidth>
          <InputLabel htmlFor="share-target-format">{t("share.field.targetFormat")}</InputLabel>
          <NativeSelect
            id="share-target-format"
            inputProps={{ "aria-describedby": "share-target-format-helper", "aria-label": t("share.field.targetFormat") }}
            name="target_format"
            defaultValue="base64"
          >
            {shareCopyFormats.map((entry) => <option key={entry.value} value={entry.value}>{entry.label}</option>)}
          </NativeSelect>
          <FormHelperText id="share-target-format-helper">{t("share.field.targetFormatHelper")}</FormHelperText>
        </FormControl>
      ) : null}
      <TextField fullWidth multiline label={t("share.ageRecipient")} minRows={2} name="age_recipient" placeholder="age1..." />
      <div className="grid gap-2">
        <Typography component="div" variant="subtitle2">{t("share.duration.label")}</Typography>
        <div aria-label={t("share.duration.customAria")} className="grid grid-cols-3 gap-2">
          <Button
            aria-pressed={durationPreset === "1d"}
            type="button"
            variant={durationPreset === "1d" ? "contained" : "outlined"}
            onClick={() => applyDurationPreset("1d")}
          >
            {t("share.duration.oneDay")}
          </Button>
          <Button
            aria-pressed={durationPreset === "7d"}
            type="button"
            variant={durationPreset === "7d" ? "contained" : "outlined"}
            onClick={() => applyDurationPreset("7d")}
          >
            {t("share.duration.sevenDays")}
          </Button>
          <Button
            aria-pressed={durationPreset === "long"}
            type="button"
            variant={durationPreset === "long" ? "contained" : "outlined"}
            onClick={() => applyDurationPreset("long")}
          >
            {t("share.duration.long")}
          </Button>
        </div>
      </div>
      <TextField label={t("share.validFrom")} name="valid_from" type="datetime-local" slotProps={{ inputLabel: { shrink: true } }} />
      <TextField
        label={t("share.validUntil")}
        name="valid_until"
        type="datetime-local"
        value={validUntil}
        slotProps={{ inputLabel: { shrink: true } }}
        onChange={(event) => {
          setDurationPreset(event.target.value ? "custom" : "long");
          setValidUntil(event.target.value);
        }}
      />
    </>
  );
}

function datetimeLocalValue(date: Date): string {
  const localTime = new Date(date.getTime() - date.getTimezoneOffset() * 60 * 1000);
  return localTime.toISOString().slice(0, 16);
}
