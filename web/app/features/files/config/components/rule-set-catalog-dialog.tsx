import { useDeferredValue, useEffect, useMemo, useState } from "react";
import Alert from "@mui/material/Alert";
import Button from "@mui/material/Button";
import CircularProgress from "@mui/material/CircularProgress";
import Dialog from "@mui/material/Dialog";
import DialogActions from "@mui/material/DialogActions";
import DialogContent from "@mui/material/DialogContent";
import DialogTitle from "@mui/material/DialogTitle";
import TextField from "@mui/material/TextField";
import Typography from "@mui/material/Typography";

import type { AddCatalogRuleSetRequest, AddCatalogRuleSetResult } from "~/features/files/config/model/editor-model";
import type { RuleSetCatalogItem, RuleSetCatalogResult, RuleSetCatalogTarget } from "~/features/files/model/types";
import { useI18n } from "~/shared/i18n/context";

const RESULT_LIMIT = 100;

export type LoadRuleSetCatalog = (target: RuleSetCatalogTarget) => Promise<RuleSetCatalogResult>;

export function RuleSetCatalogDialog({ kind, loadCatalog, onAdd, onClose, open }: {
  kind: RuleSetCatalogTarget;
  loadCatalog: LoadRuleSetCatalog;
  onAdd: (request: AddCatalogRuleSetRequest) => AddCatalogRuleSetResult;
  onClose: () => void;
  open: boolean;
}) {
  const { t } = useI18n();
  const catalogLoadFailed = t("files.config.catalogLoadFailed");
  const [search, setSearch] = useState("");
  const deferredSearch = useDeferredValue(search);
  const [catalog, setCatalog] = useState<RuleSetCatalogResult | null>(null);
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState("");
  const [actionError, setActionError] = useState("");

  useEffect(() => {
    if (open) {
      setSearch("");
      setActionError("");
    }
  }, [open]);

  useEffect(() => {
    if (!open || catalog) return;
    let active = true;
    setLoading(true);
    setLoadError("");
    void loadCatalog(kind).then((result) => {
      if (active) setCatalog(result);
    }).catch((error: unknown) => {
      if (active) setLoadError(error instanceof Error ? error.message : catalogLoadFailed);
    }).finally(() => {
      if (active) setLoading(false);
    });
    return () => { active = false; };
  }, [catalog, catalogLoadFailed, kind, loadCatalog, open]);

  const matchingItems = useMemo(() => {
    const items = catalog?.items ?? [];
    const query = deferredSearch.trim().toLocaleLowerCase();
    if (!query) return items;
    return items.filter((item) => (
      item.name.toLocaleLowerCase().includes(query) || item.url.toLocaleLowerCase().includes(query)
    ));
  }, [catalog?.items, deferredSearch]);
  const visibleItems = matchingItems.slice(0, RESULT_LIMIT);

  function addEntry(entry: RuleSetCatalogItem) {
    setActionError("");
    const result = onAdd({ entry });
    if (result.status === "added") {
      onClose();
      return;
    }
    if (result.status === "duplicate-url") {
      setActionError(t("files.config.catalogDuplicateURL", { name: result.existingName }));
      return;
    }
    setActionError(t("files.config.catalogNameConflict", { name: result.existingName }));
  }

  return (
    <Dialog fullWidth open={open} aria-labelledby="rule-set-catalog-title" maxWidth="md" onClose={onClose}>
      <DialogTitle id="rule-set-catalog-title">{t("files.config.catalogTitle")}</DialogTitle>
      <DialogContent className="grid min-w-0 gap-3" dividers>
        <TextField
          fullWidth
          label={t("actions.search")}
          slotProps={{ htmlInput: { "aria-label": t("files.config.catalogSearch") } }}
          size="small"
          value={search}
          onChange={(event) => {
            setSearch(event.target.value);
            setActionError("");
          }}
        />
        {loadError ? <Alert severity="error">{loadError}</Alert> : null}
        {actionError ? <Alert severity="warning">{actionError}</Alert> : null}
        {loading ? <div className="flex min-h-24 items-center justify-center"><CircularProgress aria-label={t("files.config.catalogLoading")} size={28} /></div> : null}
        {!loading && !loadError && !matchingItems.length ? <Typography color="text.secondary">{t("files.config.catalogEmpty")}</Typography> : null}
        {!loading && visibleItems.length ? (
          <div aria-label={t("files.config.catalogResults")} className="max-h-96 overflow-auto rounded-md border border-divider" role="list">
            {visibleItems.map((entry) => (
              <div className="grid min-w-0 gap-2 border-t border-divider p-3 first:border-t-0 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center" key={entry.url} role="listitem">
                <div className="grid min-w-0 gap-1">
                  <Typography className="font-semibold" component="span" variant="body2">{entry.name}</Typography>
                  <Typography className="break-all" color="text.secondary" component="span" variant="caption">{entry.url}</Typography>
                </div>
                <Button aria-label={`${t("files.config.catalogAdd")} “${entry.name}”`} type="button" variant="outlined" onClick={() => addEntry(entry)}>{t("actions.add")}</Button>
              </div>
            ))}
          </div>
        ) : null}
        {!loading && matchingItems.length > RESULT_LIMIT ? (
          <Typography color="text.secondary" variant="caption">{t("files.config.catalogRefineSearch", { count: RESULT_LIMIT })}</Typography>
        ) : null}
      </DialogContent>
      <DialogActions>
        <Button type="button" onClick={onClose}>{t("actions.cancel")}</Button>
      </DialogActions>
    </Dialog>
  );
}
