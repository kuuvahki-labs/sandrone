import { useCallback, useEffect, useRef, useState } from "react";

import { type Notice, type NoticeSeverity, type ShowNotice } from "./types";

export function useNotice(): { notices: Notice[]; showNotice: ShowNotice } {
  const [notices, setNotices] = useState<Notice[]>([]);
  const nextID = useRef(0);
  const timers = useRef(new Set<number>());

  useEffect(() => () => {
    timers.current.forEach((timer) => window.clearTimeout(timer));
  }, []);

  const showNotice = useCallback((message: string, severity: NoticeSeverity = "success") => {
    const notice = { id: nextID.current, message, severity };
    nextID.current += 1;
    setNotices((current) => [...current, notice]);
    const timer = window.setTimeout(() => {
      timers.current.delete(timer);
      setNotices((current) => current.filter((currentNotice) => currentNotice.id !== notice.id));
    }, 2600);
    timers.current.add(timer);
  }, []);

  return { notices, showNotice };
}
