import { useEffect, useMemo, useState } from "react";
import Button from "@mui/material/Button";
import Checkbox from "@mui/material/Checkbox";
import FormControl from "@mui/material/FormControl";
import FormControlLabel from "@mui/material/FormControlLabel";
import FormLabel from "@mui/material/FormLabel";
import Paper from "@mui/material/Paper";
import TextField from "@mui/material/TextField";
import Typography from "@mui/material/Typography";

import type { SubscriptionItem } from "~/features/subscriptions/model/types";
import { type Translator, useI18n } from "~/shared/i18n/context";

const LONG_SOURCE_LIST_THRESHOLD = 8;

export function SourceMultiSelect({ defaultValue, excludeName, subscriptions }: { defaultValue?: string[]; excludeName?: string; subscriptions: SubscriptionItem[] }) {
  const { t } = useI18n();
  const sourceItems = useMemo(() => subscriptions.filter((item) => item.name !== excludeName), [excludeName, subscriptions]);
  const sourceNames = useMemo(() => sourceItems.map((item) => item.name), [sourceItems]);
  const [query, setQuery] = useState("");
  const [selected, setSelected] = useState(() => selectedSourceSet(defaultValue, sourceNames));
  const isLongList = sourceItems.length > LONG_SOURCE_LIST_THRESHOLD;
  const normalizedQuery = query.trim().toLowerCase();
  const visibleSourceItems = normalizedQuery
    ? sourceItems.filter((item) => sourceSearchText(item).includes(normalizedQuery))
    : sourceItems;

  useEffect(() => {
    setSelected(selectedSourceSet(defaultValue, sourceNames));
  }, [defaultValue, sourceNames]);

  function toggleSource(name: string, checked: boolean) {
    setSelected((current) => {
      const next = new Set(current);
      if (checked) {
        next.add(name);
      } else {
        next.delete(name);
      }
      return next;
    });
  }

  return (
    <FormControl component="fieldset" fullWidth>
      <FormLabel component="legend">{t("subscriptions.sourcePicker.label")}</FormLabel>
      {sourceNames.filter((name) => selected.has(name)).map((name) => <input key={name} type="hidden" name="subscriptions" value={name} />)}
      {isLongList ? (
        <div className="mt-2 flex flex-col gap-2 sm:flex-row sm:items-center">
          <TextField
            fullWidth
            label={t("actions.search")}
            slotProps={{ htmlInput: { "aria-label": t("subscriptions.sourcePicker.search") } }}
            size="small"
            type="search"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
          />
          <Typography className="shrink-0" color="text.secondary" variant="body2">
            {t("subscriptions.sourcePicker.selectedCount", { selected: selected.size, total: sourceItems.length })}
          </Typography>
          <Button type="button" variant="outlined" onClick={() => setSelected(new Set(sourceNames))}>{t("subscriptions.sourcePicker.selectAll")}</Button>
          <Button type="button" onClick={() => setSelected(new Set())}>{t("subscriptions.sourcePicker.clear")}</Button>
        </div>
      ) : null}
      <div className={`mt-2 grid gap-2 ${isLongList ? "max-h-[min(42vh,320px)] overflow-y-auto pr-1" : ""}`}>
        {visibleSourceItems.length ? visibleSourceItems.map((item) => {
          const label = subscriptionKindLabel(item, t);
          const detail = item.format ? `${label} · ${item.format}` : label;

          return (
            <Paper
              key={item.name}
              variant="outlined"
              className={`min-w-0 px-3 py-1 ${selected.has(item.name) ? "border-primary" : "border-divider"}`}
            >
              <FormControlLabel
                control={(
                  <Checkbox
                    checked={selected.has(item.name)}
                    slotProps={{ input: { "aria-label": `${item.name} ${detail}` } }}
                    onChange={(event) => toggleSource(item.name, event.target.checked)}
                  />
                )}
                label={(
                  <span className="grid min-w-0 gap-0.5">
                    <Typography className="break-words" variant="body2">
                      {item.name}
                    </Typography>
                    <Typography color="text.secondary" variant="caption">
                      {detail}
                    </Typography>
                  </span>
                )}
                className="m-0 w-full items-center"
              />
            </Paper>
          );
        }) : (
          <Typography color="text.secondary" variant="body2">
            {t("subscriptions.sourcePicker.empty")}
          </Typography>
        )}
      </div>
    </FormControl>
  );
}

function sourceSearchText(item: SubscriptionItem): string {
  return [item.name, item.title, item.label, item.format].filter(Boolean).join(" ").toLowerCase();
}

function subscriptionKindLabel(item: SubscriptionItem, t: Translator): string {
  switch (item.kind) {
    case "remote":
      return t("model.subscription.remote");
    case "local":
      return t("model.subscription.local");
    case "collection":
      return t("model.subscription.collection");
  }
}

function selectedSourceSet(defaultValue: string[] | undefined, sourceNames: string[]): Set<string> {
  const allowed = new Set(sourceNames);
  return new Set((defaultValue ?? sourceNames).filter((name) => allowed.has(name)));
}
