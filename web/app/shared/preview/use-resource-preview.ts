import { useCallback, useEffect, useRef, useState } from "react";

export function useResourcePreview<TPreview>(
  resourceKey: string | undefined,
  loadPreview: () => Promise<TPreview | null>,
  refreshPreviewLoader: () => Promise<TPreview | null> = loadPreview,
) {
  const [preview, setPreview] = useState<TPreview | null>(null);
  const [pending, setPending] = useState(false);
  const [failed, setFailed] = useState(false);
  const [elapsedSeconds, setElapsedSeconds] = useState(0);
  const requestGeneration = useRef(0);

  useEffect(() => {
    if (!pending) {
      setElapsedSeconds(0);
      return;
    }

    const startedAt = Date.now();
    let timeoutID: number | undefined;
    const updateElapsedSeconds = () => {
      const elapsedMS = Date.now() - startedAt;
      setElapsedSeconds(Math.floor(elapsedMS / 1000));
      timeoutID = window.setTimeout(updateElapsedSeconds, 1000 - (elapsedMS % 1000));
    };
    setElapsedSeconds(0);
    timeoutID = window.setTimeout(updateElapsedSeconds, 1000);
    return () => window.clearTimeout(timeoutID);
  }, [pending, resourceKey]);

  const runPreview = useCallback(async (loader: () => Promise<TPreview | null>) => {
    const generation = ++requestGeneration.current;
    setPending(true);
    setFailed(false);
    try {
      const result = await loader();
      if (generation !== requestGeneration.current) return;
      setFailed(result === null);
      if (result !== null) setPreview(result);
    } catch {
      if (generation === requestGeneration.current) setFailed(true);
    } finally {
      if (generation === requestGeneration.current) setPending(false);
    }
  }, []);

  const invalidatePreview = useCallback(() => {
    requestGeneration.current++;
  }, []);

  useEffect(() => {
    setPreview(null);
    if (!resourceKey) {
      setPending(false);
      setFailed(false);
      return;
    }
    void runPreview(loadPreview);
    return invalidatePreview;
  }, [invalidatePreview, loadPreview, resourceKey, runPreview]);

  const refreshPreview = useCallback(() => {
    if (resourceKey) void runPreview(refreshPreviewLoader);
  }, [refreshPreviewLoader, resourceKey, runPreview]);

  return {
    elapsedSeconds,
    failed,
    pending,
    preview: preview ?? undefined,
    refreshPreview,
  };
}
