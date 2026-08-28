import { useEffect, useId, useState } from "react";
import FormControl from "@mui/material/FormControl";
import InputLabel from "@mui/material/InputLabel";
import Select from "@mui/material/Select";
import TextField from "@mui/material/TextField";

import { useI18n } from "~/shared/i18n/context";
import { type SnapshotCacheMode, snapshotCacheMode } from "~/shared/resources/snapshot-cache-policy";

export function SnapshotCachePolicyField({ defaultValue }: { defaultValue?: number }) {
  const { t } = useI18n();
  const labelId = useId();
  const selectId = useId();
  const [mode, setMode] = useState<SnapshotCacheMode>(() => snapshotCacheMode(defaultValue));
  const [ttlSeconds, setTTLSeconds] = useState(() => defaultValue && defaultValue > 0 ? String(defaultValue) : "");

  useEffect(() => {
    setMode(snapshotCacheMode(defaultValue));
    setTTLSeconds(defaultValue && defaultValue > 0 ? String(defaultValue) : "");
  }, [defaultValue]);

  return (
    <>
      <FormControl fullWidth>
        <InputLabel htmlFor={selectId} id={labelId}>{t("cache.snapshotPolicy")}</InputLabel>
        <Select
          native
          id={selectId}
          inputProps={{ name: "snapshot_mode" }}
          label={t("cache.snapshotPolicy")}
          labelId={labelId}
          value={mode}
          onChange={(event) => setMode(event.target.value as SnapshotCacheMode)}
        >
          <option value="inherit">{t("cache.inherit")}</option>
          <option value="disabled">{t("cache.disabled")}</option>
          <option value="custom">{t("cache.custom")}</option>
        </Select>
      </FormControl>
      {mode === "custom" ? (
        <TextField
          fullWidth
          required
          label={t("cache.snapshotTTLSeconds")}
          name="snapshot_ttl_seconds"
          slotProps={{ htmlInput: { min: 1 } }}
          type="number"
          value={ttlSeconds}
          onChange={(event) => setTTLSeconds(event.target.value)}
        />
      ) : null}
    </>
  );
}
