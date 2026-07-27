import { useEffect, useId, useState } from "react";
import FormControl from "@mui/material/FormControl";
import InputLabel from "@mui/material/InputLabel";
import Select from "@mui/material/Select";
import TextField from "@mui/material/TextField";

import { useI18n } from "~/shared/i18n/context";
import { type RenderCacheMode, renderCacheMode } from "~/shared/resources/render-cache-policy";

export function RenderCachePolicyField({ defaultValue }: { defaultValue?: number }) {
  const { t } = useI18n();
  const labelId = useId();
  const selectId = useId();
  const [mode, setMode] = useState<RenderCacheMode>(() => renderCacheMode(defaultValue));
  const [ttlSeconds, setTTLSeconds] = useState(() => defaultValue && defaultValue > 0 ? String(defaultValue) : "");

  useEffect(() => {
    setMode(renderCacheMode(defaultValue));
    setTTLSeconds(defaultValue && defaultValue > 0 ? String(defaultValue) : "");
  }, [defaultValue]);

  return (
    <>
      <FormControl fullWidth>
        <InputLabel htmlFor={selectId} id={labelId}>{t("cache.renderPolicy")}</InputLabel>
        <Select
          native
          id={selectId}
          inputProps={{ name: "render_cache_mode" }}
          label={t("cache.renderPolicy")}
          labelId={labelId}
          value={mode}
          onChange={(event) => setMode(event.target.value as RenderCacheMode)}
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
          label={t("cache.renderTTLSeconds")}
          name="render_cache_ttl_seconds"
          slotProps={{ htmlInput: { min: 1 } }}
          type="number"
          value={ttlSeconds}
          onChange={(event) => setTTLSeconds(event.target.value)}
        />
      ) : null}
    </>
  );
}
