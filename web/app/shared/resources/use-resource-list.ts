import { useCallback, useEffect, useRef, useState } from "react";

import { ApiError } from "~/shared/api/client";
import type { Translator } from "~/shared/i18n/context";

export type ResourceErrorNotice = (message: string, severity: "error") => void;

export interface ResourceListState<T> {
  items: T[];
  loaded: boolean;
  loading: boolean;
  reload: () => Promise<void>;
}

export function useResourceList<T>({
  load,
  map,
  cached,
  showNotice,
  t,
}: {
  load: (options?: { fresh?: boolean }) => Promise<unknown>;
  cached?: () => { value: unknown; fresh: boolean } | undefined;
  map: (resourceList: unknown) => T[];
  showNotice: ResourceErrorNotice;
  t: Translator;
}): ResourceListState<T> {
  const generation = useRef(0);
  const [initial] = useState(() => cached?.());
  const [items, setItems] = useState<T[]>(() => initial ? map(initial.value) : []);
  const [loaded, setLoaded] = useState(!!initial);
  const [loading, setLoading] = useState(!initial);

  const refresh = useCallback(async (fresh: boolean) => {
    if (!fresh && cached?.()?.fresh) return;
    const currentGeneration = generation.current + 1;
    generation.current = currentGeneration;
    setLoading(true);
    try {
      const nextItems = map(await load(fresh ? { fresh: true } : undefined));
      if (generation.current === currentGeneration) {
        setItems(nextItems);
        setLoaded(true);
      }
    } catch (error) {
      if (generation.current === currentGeneration) {
        if (!(error instanceof ApiError && (error.status === 401 || error.code === "stale_session"))) {
          showNotice(error instanceof Error ? error.message : t("errors.serviceUnavailable"), "error");
        }
      }
    } finally {
      if (generation.current === currentGeneration) {
        setLoading(false);
      }
    }
  }, [cached, load, map, showNotice, t]);

  const reload = useCallback(() => refresh(true), [refresh]);

  useEffect(() => {
    const previous = cached?.();
    if (previous) {
      setItems(map(previous.value));
      setLoaded(true);
      setLoading(false);
    }
    void refresh(false);
    const onFocus = () => { void refresh(false); };
    window.addEventListener("focus", onFocus);
    return () => {
      generation.current += 1;
      window.removeEventListener("focus", onFocus);
    };
  }, [cached, map, refresh]);

  return { items, loaded, loading, reload };
}
