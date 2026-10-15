import { useCallback, useEffect, useRef, useState } from "react";
import { BASE_URL } from "@/lib/api/client";
import { authStore } from "@/stores/auth";
import { openDocsSession } from "./api";

/** DocsProblem is why the reference did not open: the browser refused the tab, or the session could not be made. */
export type DocsProblem = "blocked" | "failed";

/**
 * useDocsLauncher opens the API reference in a new tab. The tab is opened
 * inside the click, with its opener cleared, then pointed at `/docs` (at the
 * `#tag/{tag}` of a group when it is given) once the docs session exists, and
 * closed if it cannot be made. A second call while one is out does nothing.
 * `retry` opens again at the group the last `open` asked for. `open` and
 * `retry` are stable.
 */
export function useDocsLauncher() {
  const [problem, setProblem] = useState<DocsProblem | null>(null);
  const [pending, setPending] = useState(false);
  const mounted = useRef(true);
  const running = useRef(false);
  const asked = useRef<string | undefined>(undefined);
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
    };
  }, []);

  const open = useCallback((tag?: string) => {
    if (running.current) return;
    asked.current = tag;
    const lease = authStore.captureActor();
    const tab = window.open("about:blank", "_blank");
    if (!tab) {
      setProblem("blocked");
      return;
    }
    tab.opener = null;
    running.current = true;
    setProblem(null);
    setPending(true);
    const here = () => mounted.current && authStore.isCurrent(lease);
    openDocsSession()
      .then(
        () => {
          if (!here()) {
            tab.close();
            return;
          }
          tab.location.href = `${BASE_URL}/docs${
            tag === undefined ? "" : `#tag/${encodeURIComponent(tag)}`
          }`;
        },
        () => {
          tab.close();
          if (here()) setProblem("failed");
        },
      )
      .finally(() => {
        running.current = false;
        if (here()) setPending(false);
      });
  }, []);

  const retry = useCallback(() => open(asked.current), [open]);

  return { open, retry, pending, problem };
}
