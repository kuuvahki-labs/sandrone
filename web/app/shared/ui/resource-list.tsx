import { type ReactNode, useId, useState } from "react";
import MoreVertIcon from "@mui/icons-material/MoreVert";
import IconButton from "@mui/material/IconButton";
import ListItem from "@mui/material/ListItem";
import ListItemButton from "@mui/material/ListItemButton";
import ListItemIcon from "@mui/material/ListItemIcon";
import ListItemText from "@mui/material/ListItemText";
import Menu from "@mui/material/Menu";
import MenuItem from "@mui/material/MenuItem";
import Paper from "@mui/material/Paper";
import SpeedDial from "@mui/material/SpeedDial";
import SpeedDialAction from "@mui/material/SpeedDialAction";
import SpeedDialIcon from "@mui/material/SpeedDialIcon";
import Tooltip from "@mui/material/Tooltip";
import Typography from "@mui/material/Typography";

import { useI18n } from "~/shared/i18n/context";

export interface DestinationListAction {
  accessibleLabel?: string;
  disabled?: boolean;
  disabledReason?: string;
  icon?: ReactNode;
  label: string;
  onSelect: () => void;
  tone?: "default" | "danger";
}

export function ActionMenu({ actions, buttonSize, label }: { actions: DestinationListAction[]; buttonSize?: "small" | "medium" | "large"; label: string }) {
  const { t } = useI18n();
  const [anchor, setAnchor] = useState<HTMLElement | null>(null);
  const menuId = useId();
  if (actions.length === 0) {
    return null;
  }
  return (
    <>
      <Tooltip title={t("actions.more")}>
        <IconButton
          aria-label={label}
          aria-controls={anchor ? menuId : undefined}
          aria-haspopup="menu"
          aria-expanded={anchor ? "true" : undefined}
          size={buttonSize}
          type="button"
          onClick={(event) => setAnchor(event.currentTarget)}
        >
          <MoreVertIcon aria-hidden />
        </IconButton>
      </Tooltip>
      <Menu
        id={menuId}
        anchorEl={anchor}
        open={Boolean(anchor)}
        slotProps={{ list: { disabledItemsFocusable: true } }}
        onClose={() => setAnchor(null)}
      >
        {actions.map((action) => {
          const key = action.accessibleLabel ?? action.label;
          const item = (
            <MenuItem
              aria-description={action.disabledReason}
              aria-disabled={action.disabled || undefined}
              aria-label={action.accessibleLabel}
              disabled={action.disabled && !action.disabledReason}
              key={key}
              sx={{
                ...(action.tone === "danger" ? { color: "error.main" } : {}),
                ...(action.disabled ? { color: "text.disabled", cursor: "default" } : {}),
              }}
              onClick={(event) => {
                if (action.disabled) {
                  event.preventDefault();
                  return;
                }
                setAnchor(null);
                action.onSelect();
              }}
            >
              {action.icon}
              {action.label}
            </MenuItem>
          );
          return action.disabledReason ? (
            <Tooltip describeChild key={key} placement="left" title={action.disabledReason}>
              {item}
            </Tooltip>
          ) : item;
        })}
      </Menu>
    </>
  );
}

export function DestinationListItem({
  actions,
  actionTitle,
  details,
  icon,
  meta,
  onPrimaryAction,
  primaryLabel,
  subtitle,
  title,
}: {
  actions?: DestinationListAction[];
  actionTitle?: string;
  details?: ReactNode;
  icon: ReactNode;
  meta?: ReactNode;
  onPrimaryAction: () => void;
  primaryLabel: string;
  subtitle?: ReactNode;
  title: string;
}) {
  const { t } = useI18n();
  const effectiveActionTitle = actionTitle || title;
  const hasSubtitle = subtitle !== undefined && subtitle !== null && subtitle !== "";
  const secondary = hasSubtitle || meta ? (
    <div className="grid min-w-0 gap-2">
      {hasSubtitle ? (
        <Typography className="break-words" color="text.secondary" component="span" variant="body2">
          {subtitle}
        </Typography>
      ) : null}
      {meta ? <div className="flex flex-wrap items-center gap-2">{meta}</div> : null}
    </div>
  ) : undefined;

  return (
    <ListItem disablePadding className="destination-list-item">
      <Paper variant="outlined" className="w-full overflow-hidden">
        <div className="grid gap-1">
          <div className="flex min-w-0 items-stretch gap-1 p-1">
            <ListItemButton
              aria-label={t("resourceList.primaryAction", { action: primaryLabel, title: effectiveActionTitle })}
              className="min-w-0 flex-1 items-start rounded-lg px-3 py-3"
              onClick={onPrimaryAction}
            >
              <ListItemIcon>{icon}</ListItemIcon>
              <ListItemText
                className="m-0 min-w-0"
                slotProps={{ secondary: { component: "div" } }}
                primary={(
                  <Typography className="break-words" component="h3" variant="h6">
                    {title}
                  </Typography>
                )}
                secondary={secondary}
              />
            </ListItemButton>
            <div className="flex items-start pt-2">
              <ActionMenu actions={actions ?? []} label={t("resourceList.moreActions", { title: effectiveActionTitle })} />
            </div>
          </div>
          {details ? <div className="px-4 pb-4">{details}</div> : null}
        </div>
      </Paper>
    </ListItem>
  );
}

export interface CreateSpeedDialAction {
  ariaLabel?: string;
  icon: ReactNode;
  label: string;
  onSelect: () => void;
}

export function CreateSpeedDial({ actions, ariaLabel }: { actions: CreateSpeedDialAction[]; ariaLabel: string }) {
  const [open, setOpen] = useState(false);
  if (actions.length === 0) {
    return null;
  }

  return (
    <SpeedDial
      ariaLabel={ariaLabel}
      className="fixed right-4 bottom-[88px] z-[1050] sm:right-6 md:bottom-6"
      direction="up"
      icon={<SpeedDialIcon />}
      open={open}
      onClose={(_event, reason) => {
        if (reason === "toggle" || reason === "escapeKeyDown") {
          setOpen(false);
        }
      }}
      onOpen={(_event, reason) => {
        if (reason === "toggle") {
          setOpen(true);
        }
      }}
    >
      {open ? actions.map((action) => (
        <SpeedDialAction
          icon={action.icon}
          key={action.label}
          slotProps={{
            fab: { "aria-label": action.ariaLabel ?? action.label },
            tooltip: { title: action.label },
          }}
          onClick={() => {
            setOpen(false);
            action.onSelect();
          }}
        />
      )) : null}
    </SpeedDial>
  );
}
