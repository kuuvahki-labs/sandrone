import { useDeferredValue, useEffect, useMemo, useState } from "react";
import Alert from "@mui/material/Alert";
import Button from "@mui/material/Button";
import CircularProgress from "@mui/material/CircularProgress";
import Dialog from "@mui/material/Dialog";
import DialogActions from "@mui/material/DialogActions";
import DialogContent from "@mui/material/DialogContent";
import DialogTitle from "@mui/material/DialogTitle";
import Pagination from "@mui/material/Pagination";
import TextField from "@mui/material/TextField";
import Typography from "@mui/material/Typography";

import type { AddCatalogRuleSetRequest, AddCatalogRuleSetResult } from "~/features/files/config/model/editor-model";
import type { RuleSetCatalogItem, RuleSetCatalogResult, RuleSetCatalogTarget } from "~/features/files/model/types";
import { useI18n } from "~/shared/i18n/context";

import { deriveCatalogView } from "./rule-set-catalog-view";

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
  const [page, setPage] = useState(1);
  const [catalog, setCatalog] = useState<RuleSetCatalogResult | null>(null);
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState("");
  const [actionError, setActionError] = useState("");

  useEffect(() => {
    if (open) {
      setSearch("");
      setPage(1);
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

  const catalogView = useMemo(() => deriveCatalogView({
    items: catalog?.items ?? [],
    page,
    query: deferredSearch,
  }), [catalog?.items, deferredSearch, page]);

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
            setPage(1);
            setActionError("");
          }}
        />
        {loadError ? <Alert severity="error">{loadError}</Alert> : null}
        {actionError ? <Alert severity="warning">{actionError}</Alert> : null}
        {loading ? <div className="flex min-h-24 items-center justify-center"><CircularProgress aria-label={t("files.config.catalogLoading")} size={28} /></div> : null}
        {!loading && !loadError ? <Typography color="text.secondary" variant="caption">{t("files.config.catalogResultCount", { count: catalogView.total })}</Typography> : null}
        {!loading && !loadError && !catalogView.total ? <Typography color="text.secondary">{t("files.config.catalogEmpty")}</Typography> : null}
        {!loading && catalogView.items.length ? (
          <div aria-label={t("files.config.catalogResults")} className="max-h-96 overflow-auto rounded-md border border-divider" role="list">
            {catalogView.items.map((entry) => (
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
        {!loading && !loadError && catalogView.pageCount > 1 ? (
          <div className="flex justify-center">
            <Pagination aria-label={t("files.config.catalogPages")} count={catalogView.pageCount} page={page} size="small" onChange={(_event, nextPage) => setPage(nextPage)} />
          </div>
        ) : null}
      </DialogContent>
      <DialogActions>
        <Button type="button" onClick={onClose}>{t("actions.cancel")}</Button>
      </DialogActions>
    </Dialog>
  );
}
