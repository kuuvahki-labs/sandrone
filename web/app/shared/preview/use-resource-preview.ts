import { useCallback, useEffect, useState } from "react";

export function useResourcePreview<TPreview>(
  resourceKey: string | undefined,
  loadPreview: () => Promise<TPreview | null>,
  refreshPreviewLoader: () => Promise<TPreview | null> = loadPreview,
) {
  const [preview, setPreview] = useState<TPreview | null>(null);
  const [pending, setPending] = useState(false);
  const [failed, setFailed] = useState(false);
  const [elapsedSeconds, setElapsedSeconds] = useState(0);

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

  useEffect(() => {
    if (!resourceKey) {
      setPreview(null);
      setPending(false);
      setFailed(false);
      return;
    }
    let active = true;
    setPreview(null);
    setPending(true);
    setFailed(false);
    void loadPreview().then((result) => {
      if (active) {
        setPreview(result);
        setFailed(result === null);
      }
    }).finally(() => {
      if (active) {
        setPending(false);
      }
    });
    return () => {
      active = false;
    };
  }, [loadPreview, resourceKey]);

  const refreshPreview = useCallback(() => {
    if (!resourceKey) {
      return;
    }
    setPending(true);
    setFailed(false);
    void refreshPreviewLoader().then((result) => {
      if (result === null) {
        setFailed(true);
        return;
      }
      setPreview(result);
    }).finally(() => {
      setPending(false);
    });
  }, [refreshPreviewLoader, resourceKey]);

  return {
    elapsedSeconds,
    failed,
    pending,
    preview: preview ?? undefined,
    refreshPreview,
  };
}
