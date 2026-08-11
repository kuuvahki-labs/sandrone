import { useCallback, useEffect, useState } from "react";

export function useResourcePreview<TPreview>(
  resourceKey: string | undefined,
  loadPreview: () => Promise<TPreview | null>,
  refreshPreviewLoader: () => Promise<TPreview | null> = loadPreview,
) {
  const [preview, setPreview] = useState<TPreview | null>(null);
  const [pending, setPending] = useState(false);
  const [failed, setFailed] = useState(false);

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
    failed,
    pending,
    preview: preview ?? undefined,
    refreshPreview,
  };
}
