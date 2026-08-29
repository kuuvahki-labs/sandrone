import { useEffect, useState } from "react";

import type { ApiClient } from "~/shared/api/client";

export function useVersionInfo({ client }: { client: ApiClient }) {
  const [buildTime, setBuildTime] = useState<string>();
  const [name, setName] = useState<string>();
  const [version, setVersion] = useState<string>();
  const [revision, setRevision] = useState<string>();

  useEffect(() => {
    let cancelled = false;
    void client.getVersion()
      .then((info) => {
        if (!cancelled) {
          setBuildTime(info.build_time);
          setName(info.name);
          setVersion(info.version);
          setRevision(info.revision);
        }
      })
      .catch(() => undefined);
    return () => {
      cancelled = true;
    };
  }, [client]);

  return { buildTime, name, revision, version };
}
