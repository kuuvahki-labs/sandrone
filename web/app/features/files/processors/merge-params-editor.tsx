import { useId } from "react";
import ErrorOutlinedIcon from "@mui/icons-material/ErrorOutlined";
import FormControl from "@mui/material/FormControl";
import IconButton from "@mui/material/IconButton";
import InputLabel from "@mui/material/InputLabel";
import MenuItem from "@mui/material/MenuItem";
import Select from "@mui/material/Select";
import Tooltip from "@mui/material/Tooltip";
import Typography from "@mui/material/Typography";

import type { FileMergeMode } from "~/features/files/drivers/core/file-driver";
import { requireFileDriver } from "~/features/files/drivers/registry";
import type { FileKind } from "~/features/files/model/types";
import { type Translator, useI18n } from "~/shared/i18n/context";
import { HighlightedTextarea } from "~/shared/ui/code-editor";

export function FileMergeParamsEditor({
  kind,
  onChange,
  params,
}: {
  kind: FileKind;
  onChange: (patch: Record<string, unknown>) => void;
  params: Record<string, unknown>;
}) {
  const { t } = useI18n();
  const modeLabelID = useId();
  const content = typeof params.content === "string" ? params.content : "";
  const driver = requireFileDriver(kind);
  const mode = typeof params.mode === "string" && driver.processors.mergeModes.includes(params.mode as FileMergeMode)
    ? params.mode as FileMergeMode
    : driver.processors.mergeModes[0] ?? "yaml_overlay";
  const isOverride = mode === "yaml_override" || mode === "json_override" || mode === "ini_override";
  const isOverlay = mode === "yaml_overlay" || mode === "json_overlay";
  return (
    <>
      {driver.processors.mergeModes.length ? (
        <FormControl fullWidth>
          <InputLabel id={modeLabelID}>{t("processor.merge.mode")}</InputLabel>
          <Select label={t("processor.merge.mode")} labelId={modeLabelID} value={mode} onChange={(event) => onChange({ mode: event.target.value })}>
            {driver.processors.mergeModes.map((option) => <MenuItem key={option} value={option}>{mergeModeLabel(option, t)}</MenuItem>)}
          </Select>
        </FormControl>
      ) : null}
      <div className="grid gap-1.5 md:col-span-2">
        <HighlightedTextarea
          label={t("processor.merge.content")}
          labelAction={isOverride ? (
            <Tooltip title={t(mode === "ini_override" ? "processor.merge.iniOverrideDescription" : "processor.merge.overrideDescription")}>
              <IconButton aria-label={t("processor.merge.syntaxHelp")} size="small" type="button">
                <ErrorOutlinedIcon aria-hidden="true" fontSize="small" />
              </IconButton>
            </Tooltip>
          ) : undefined}
          language={driver.source.syntax}
          minRows={6}
          placeholder={mergePlaceholder(driver.source.syntax)}
          value={content}
          onChange={(event) => onChange({ content: event.target.value })}
        />
        {isOverlay ? (
          <Typography color="text.secondary" variant="caption">{t("processor.merge.description")}</Typography>
        ) : null}
      </div>
    </>
  );
}

function mergeModeLabel(mode: FileMergeMode, t: Translator): string {
  const keys = {
    yaml_overlay: "processor.merge.mode.yamlOverlay",
    yaml_override: "processor.merge.mode.yamlOverride",
    json_overlay: "processor.merge.mode.jsonOverlay",
    json_override: "processor.merge.mode.jsonOverride",
    ini_override: "processor.merge.mode.iniOverride",
  } as const;
  return t(keys[mode]);
}

function mergePlaceholder(syntax: "text" | "yaml" | "json" | "ini"): string {
  if (syntax === "json") return "{\n  \"log\": { \"level\": \"debug\" }\n}";
  if (syntax === "ini") return "[General]\nloglevel = notify\n\n[Rule+]\nFINAL,DIRECT";
  return "mode: global";
}
