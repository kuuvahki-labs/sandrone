import { useEffect, useState } from "react";

import type { ApiClient } from "~/shared/api/client";

export function useVersionInfo({ client }: { client: ApiClient }) {
  const [version, setVersion] = useState<string>();
  const [revision, setRevision] = useState<string>();

  useEffect(() => {
    let cancelled = false;
    void client.getVersion()
      .then((info) => {
        if (!cancelled) {
          setVersion(info.version);
          setRevision(info.revision);
        }
      })
      .catch(() => undefined);
    return () => {
      cancelled = true;
    };
  }, [client]);

  return { revision, version };
}
