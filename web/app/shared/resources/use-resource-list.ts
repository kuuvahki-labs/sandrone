import { useCallback, useEffect, useRef, useState } from "react";

import { ApiError } from "~/shared/api/client";
import type { Translator } from "~/shared/i18n/context";

export type ResourceErrorNotice = (message: string, severity: "error") => void;

export interface ResourceListState<T> {
  items: T[];
  loading: boolean;
  reload: () => Promise<void>;
}

export function useResourceList<T>({
  load,
  map,
  showNotice,
  t,
}: {
  load: () => Promise<unknown>;
  map: (resourceList: unknown) => T[];
  showNotice: ResourceErrorNotice;
  t: Translator;
}): ResourceListState<T> {
  const generation = useRef(0);
  const [items, setItems] = useState<T[]>([]);
  const [loading, setLoading] = useState(true);

  const reload = useCallback(async () => {
    const currentGeneration = generation.current + 1;
    generation.current = currentGeneration;
    setLoading(true);
    try {
      const nextItems = map(await load());
      if (generation.current === currentGeneration) {
        setItems(nextItems);
      }
    } catch (error) {
      if (generation.current === currentGeneration && !(error instanceof ApiError && error.status === 401)) {
        setItems([]);
        showNotice(error instanceof Error ? error.message : t("errors.serviceUnavailable"), "error");
      }
    } finally {
      if (generation.current === currentGeneration) {
        setLoading(false);
      }
    }
  }, [load, map, showNotice, t]);

  useEffect(() => {
    void reload();
    return () => {
      generation.current += 1;
    };
  }, [reload]);

  return { items, loading, reload };
}
