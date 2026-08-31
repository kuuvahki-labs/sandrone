import DeleteOutlineIcon from "@mui/icons-material/DeleteOutlineOutlined";
import Button from "@mui/material/Button";
import Card from "@mui/material/Card";
import CardContent from "@mui/material/CardContent";
import IconButton from "@mui/material/IconButton";
import List from "@mui/material/List";
import ListItem from "@mui/material/ListItem";
import ListItemText from "@mui/material/ListItemText";
import Typography from "@mui/material/Typography";

import { useI18n } from "~/shared/i18n/context";
import type { IgnoredWarning } from "~/shared/resources/types";
import { warningIgnoreKey } from "~/shared/resources/warning-groups";

interface IgnoredWarningsSettingsSectionProps {
  value: readonly IgnoredWarning[];
  onChange: (value: IgnoredWarning[]) => void;
}

export function IgnoredWarningsSettingsSection({ value, onChange }: IgnoredWarningsSettingsSectionProps) {
  const { t } = useI18n();
  const groups = groupIgnoredWarnings(value);
  return (
    <Card component="article" variant="outlined">
      <CardContent className="grid gap-3">
        <div className="flex flex-wrap items-start justify-between gap-2">
          <div className="grid gap-1">
            <Typography component="h3" variant="h6">{t("settings.ignoredWarnings.title")}</Typography>
            <Typography color="text.secondary" variant="body2">{t("settings.ignoredWarnings.description")}</Typography>
          </div>
          {value.length ? (
            <Button color="error" size="small" type="button" onClick={() => onChange([])}>
              {t("settings.ignoredWarnings.restoreAll")}
            </Button>
          ) : null}
        </div>
        {value.length ? (
          <List className="p-0">
            {groups.map((group) => {
              return (
                <ListItem
                  disableGutters
                  key={group.key}
                  secondaryAction={(
                    <IconButton
                      aria-label={t("settings.ignoredWarnings.restoreOne", { warning: group.label })}
                      edge="end"
                      onClick={() => onChange(value.filter((item) => ignoredWarningGroupKey(item) !== group.key))}
                    >
                      <DeleteOutlineIcon aria-hidden fontSize="small" />
                    </IconButton>
                  )}
                >
                  <ListItemText
                    primary={group.label}
                    secondary={group.field
                      ? t("settings.ignoredWarnings.fieldRuleCount", { count: group.warnings.length })
                      : warningGroupContext(group.warnings) || t("settings.ignoredWarnings.codeOnly")}
                  />
                </ListItem>
              );
            })}
          </List>
        ) : (
          <Typography color="text.secondary" variant="body2">{t("settings.ignoredWarnings.empty")}</Typography>
        )}
      </CardContent>
    </Card>
  );
}

interface IgnoredWarningGroup {
  field?: string;
  key: string;
  label: string;
  warnings: IgnoredWarning[];
}

function groupIgnoredWarnings(warnings: readonly IgnoredWarning[]): IgnoredWarningGroup[] {
  const groups: IgnoredWarningGroup[] = [];
  const groupsByKey = new Map<string, IgnoredWarningGroup>();
  for (const warning of warnings) {
    const key = ignoredWarningGroupKey(warning);
    const existing = groupsByKey.get(key);
    if (existing) {
      existing.warnings.push(warning);
      continue;
    }
    const group = {
      ...(warning.field ? { field: warning.field } : {}),
      key,
      label: warning.field || warning.code,
      warnings: [warning],
    };
    groupsByKey.set(key, group);
    groups.push(group);
  }
  return groups;
}

function ignoredWarningGroupKey(warning: IgnoredWarning): string {
  return warning.field ? `field:${warning.field}` : `warning:${warningIgnoreKey(warning)}`;
}

function warningGroupContext(warnings: readonly IgnoredWarning[]): string {
  return warnings.map((warning) => [
    warning.field ? warning.code : "",
    warning.source ? `source: ${warning.source}` : "",
    warning.target ? `target: ${warning.target}` : "",
  ].filter(Boolean).join(" · ")).join(" / ");
}
