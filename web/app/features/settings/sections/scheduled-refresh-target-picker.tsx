import { useId, useState } from "react";
import KeyboardArrowDownIcon from "@mui/icons-material/KeyboardArrowDown";
import KeyboardArrowUpIcon from "@mui/icons-material/KeyboardArrowUp";
import Button from "@mui/material/Button";
import ButtonBase from "@mui/material/ButtonBase";
import Checkbox from "@mui/material/Checkbox";
import Collapse from "@mui/material/Collapse";
import FormControl from "@mui/material/FormControl";
import FormControlLabel from "@mui/material/FormControlLabel";
import FormLabel from "@mui/material/FormLabel";
import Paper from "@mui/material/Paper";
import TextField from "@mui/material/TextField";
import Typography from "@mui/material/Typography";

import {
  type ScheduledRefreshResourceChoice,
  scheduledRefreshTargetOptions,
  targetKey,
} from "~/features/settings/model/scheduled-refresh-targets";
import type { ScheduledRefreshTarget } from "~/shared/api/client";
import { useI18n } from "~/shared/i18n/context";

export function ScheduledRefreshTargetPicker({ targets, resources, onChange }: {
  targets: ScheduledRefreshTarget[];
  resources: ScheduledRefreshResourceChoice[];
  onChange: (targets: ScheduledRefreshTarget[]) => void;
}) {
  const { t } = useI18n();
  const [expanded, setExpanded] = useState(false);
  const [query, setQuery] = useState("");
  const id = useId();
  const options = scheduledRefreshTargetOptions(targets, resources);
  const selectedKeys = new Set(targets.map(targetKey));
  const missingCount = options.filter((option) => option.missing).length;
  const normalizedQuery = query.trim().toLowerCase();
  const visibleOptions = options.filter((option) =>
    `${option.label}\n${option.name}`.toLowerCase().includes(normalizedQuery),
  );
  const additions = visibleOptions.filter((option) => !option.missing && !selectedKeys.has(targetKey(option)));

  return (
    <Paper className="min-w-0 overflow-hidden" variant="outlined">
      <ButtonBase
        aria-label={t("settings.scheduledRefresh.targets")}
        aria-describedby={`${id}-count${missingCount ? ` ${id}-missing` : ""}`}
        aria-expanded={expanded}
        aria-controls={`${id}-options`}
        className="w-full gap-2 p-3 text-left"
        type="button"
        onClick={() => setExpanded((current) => !current)}
      >
        <span className="flex min-w-0 flex-1 flex-wrap items-center justify-between gap-x-3 gap-y-1">
          <Typography component="span" variant="body2">{t("settings.scheduledRefresh.targets")}</Typography>
          <Typography color="text.secondary" component="span" id={`${id}-count`} role="status" variant="body2">
            {t("settings.scheduledRefresh.selectedCount", { count: selectedKeys.size })}
          </Typography>
        </span>
        {expanded ? <KeyboardArrowUpIcon aria-hidden fontSize="small" /> : <KeyboardArrowDownIcon aria-hidden fontSize="small" />}
      </ButtonBase>
      {missingCount > 0 ? (
        <Typography className="px-3 pb-2" color="warning.main" id={`${id}-missing`} variant="body2">
          {t("settings.scheduledRefresh.missingCount", { count: missingCount })}
        </Typography>
      ) : null}
      <Collapse id={`${id}-options`} in={expanded} unmountOnExit>
        <div className="grid min-w-0 gap-2 border-t border-divider p-3">
          <TextField
            fullWidth
            label={t("settings.scheduledRefresh.searchTargets")}
            size="small"
            type="search"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
          />
          <div className="flex flex-wrap items-center justify-between gap-1">
            <Button
              disabled={additions.length === 0}
              size="small"
              type="button"
              variant="outlined"
              onClick={() => onChange([...targets, ...additions.map(({ kind, name }) => ({ kind, name }))])}
            >
              {t("settings.scheduledRefresh.selectVisible")}
            </Button>
            <Button disabled={targets.length === 0} size="small" type="button" onClick={() => onChange([])}>
              {t("settings.scheduledRefresh.clearTargets")}
            </Button>
          </div>
          <div className="grid max-h-[min(42vh,320px)] min-w-0 gap-3 overflow-y-auto pr-1">
            {(["subscription", "file"] as const).map((kind) => {
              const group = visibleOptions.filter((option) => option.kind === kind);
              if (!group.length) return null;
              return (
                <FormControl className="min-w-0" component="fieldset" fullWidth key={kind}>
                  <FormLabel component="legend">
                    {t(kind === "subscription" ? "settings.scheduledRefresh.groupSubscriptions" : "settings.scheduledRefresh.groupFiles")}
                  </FormLabel>
                  <div className="mt-2 grid min-w-0 gap-2">
                    {group.map((option) => {
                      const key = targetKey(option);
                      const checked = selectedKeys.has(key);
                      return (
                        <Paper className={`min-w-0 px-2 py-1 ${checked ? "border-primary" : "border-divider"}`} key={key} variant="outlined">
                          <FormControlLabel
                            className="m-0 w-full items-center"
                            control={(
                              <Checkbox
                                checked={checked}
                                onChange={(event) => onChange(event.target.checked
                                  ? [...targets, { kind: option.kind, name: option.name }]
                                  : targets.filter((target) => targetKey(target) !== key))}
                              />
                            )}
                            label={(
                              <span className="grid min-w-0 gap-0.5">
                                <Typography className="break-words [overflow-wrap:anywhere]" component="span" variant="body2">{option.label}</Typography>
                                {option.missing ? (
                                  <Typography color="warning.main" component="span" variant="caption">{t("settings.scheduledRefresh.missing")}</Typography>
                                ) : null}
                              </span>
                            )}
                            slotProps={{ typography: { className: "min-w-0" } }}
                          />
                        </Paper>
                      );
                    })}
                  </div>
                </FormControl>
              );
            })}
            {visibleOptions.length === 0 ? (
              <Typography color="text.secondary" variant="body2">
                {t(options.length === 0 ? "settings.scheduledRefresh.noTargets" : "settings.scheduledRefresh.noMatchingTargets")}
              </Typography>
            ) : null}
          </div>
        </div>
      </Collapse>
    </Paper>
  );
}
