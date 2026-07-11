import type { ReactNode } from "react";
import AccountTreeOutlinedIcon from "@mui/icons-material/AccountTreeOutlined";
import DescriptionOutlinedIcon from "@mui/icons-material/DescriptionOutlined";
import SettingsOutlinedIcon from "@mui/icons-material/SettingsOutlined";
import ShareOutlinedIcon from "@mui/icons-material/ShareOutlined";
import BottomNavigation from "@mui/material/BottomNavigation";
import BottomNavigationAction from "@mui/material/BottomNavigationAction";
import Container from "@mui/material/Container";
import Drawer from "@mui/material/Drawer";
import List from "@mui/material/List";
import ListItemButton from "@mui/material/ListItemButton";
import ListItemIcon from "@mui/material/ListItemIcon";
import ListItemText from "@mui/material/ListItemText";
import Typography from "@mui/material/Typography";

import { type TranslationKey, useI18n } from "~/shared/i18n/context";

import { BrandLogo } from "./brand-logo";

const navItems = [
  { path: "/subscriptions", labelKey: "nav.subscriptions", icon: AccountTreeOutlinedIcon },
  { path: "/files", labelKey: "nav.files", icon: DescriptionOutlinedIcon },
  { path: "/shares", labelKey: "nav.shares", icon: ShareOutlinedIcon },
  { path: "/settings", labelKey: "nav.settings", icon: SettingsOutlinedIcon },
] satisfies Array<{ icon: typeof AccountTreeOutlinedIcon; labelKey: TranslationKey; path: string }>;

interface ShellFrameProps {
  activePath: string;
  children: ReactNode;
}

export function ShellFrame({ activePath, children }: ShellFrameProps) {
  const { t } = useI18n();
  const active = activeNav(activePath);
  const focusMode = isFocusModePath(activePath);
  return (
    <main className="min-h-screen bg-background-default min-[820px]:flex">
      <Drawer
        open
        className="desktop-drawer hidden min-[820px]:block min-[820px]:w-60 min-[820px]:shrink-0"
        slotProps={{
          paper: {
            className: "w-60 border-r border-divider bg-background-paper px-2 py-3",
          },
        }}
        variant="permanent"
      >
        <div className="flex items-center gap-3 px-2 py-2">
          <BrandLogo size={48} src="/brand/sandrone-logo-48.png" />
          <Typography component="div" variant="h6">
            Sandrone
          </Typography>
        </div>
        <List component="nav" aria-label={t("nav.desktop")} className="grid gap-1 p-0">
          {navItems.map((item) => {
            const Icon = item.icon;
            const label = t(item.labelKey);
            return (
              <ListItemButton
                href={item.path}
                key={item.path}
                selected={active.path === item.path}
                className="gap-3 px-3 py-2"
              >
                <ListItemIcon className="min-w-10">
                  <Icon aria-hidden />
                </ListItemIcon>
                <ListItemText
                  className="m-0"
                  primary={<Typography component="span">{label}</Typography>}
                />
              </ListItemButton>
            );
          })}
        </List>
      </Drawer>
      <section className={`min-w-0 flex-grow px-4 py-4 min-[820px]:px-6 min-[820px]:py-6 ${focusMode ? "pb-6" : "pb-24 min-[820px]:pb-6"}`}>
        <Container disableGutters maxWidth="lg">
          {children}
        </Container>
      </section>
      {focusMode ? null : (
        <nav
          aria-label={t("nav.bottom")}
          className="bottom-nav fixed inset-x-0 bottom-0 z-[1100] block bg-background-paper shadow-3 min-[820px]:hidden"
        >
          <BottomNavigation
            showLabels
            value={active.path}
          >
            {navItems.map((item) => {
              const Icon = item.icon;
              return (
                <BottomNavigationAction
                  href={item.path}
                  key={item.path}
                  icon={<Icon aria-hidden />}
                  label={t(item.labelKey)}
                  value={item.path}
                />
              );
            })}
          </BottomNavigation>
        </nav>
      )}
    </main>
  );
}

function activeNav(path: string) {
  if (path === "/" || path.startsWith("/subscriptions")) {
    return navItems[0];
  }
  return navItems.find((item) => path.startsWith(item.path)) ?? navItems[0];
}

function isFocusModePath(path: string) {
  return path.endsWith("/edit") || path.endsWith("/new") || path.endsWith("/preview");
}
