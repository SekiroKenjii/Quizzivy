import { useCallback, useEffect, useRef } from "react";
import { useSearchParams } from "react-router";
import { parsePage } from "@/lib/pagination";

/**
 * The current page, kept in the URL (`?page=3`) so it survives a reload and
 * can be shared. Every other search parameter is preserved; page 1 is the
 * absence of the parameter, so the first page's URL is the plain route.
 *
 * `filters` is whatever else shapes the list. When it changes the page goes
 * back to 1 -- page 7 of one search is not page 7 of another -- but not on
 * mount, so a shared `?page=3` link still opens on page 3.
 */
export function usePage(filters = ""): [number, (page: number) => void] {
  const [params, setParams] = useSearchParams();
  const page = parsePage(params.get("page"));
  const setPage = useCallback(
    (next: number) => {
      setParams(
        (current) => {
          const out = new URLSearchParams(current);
          if (next <= 1) out.delete("page");
          else out.set("page", String(next));
          return out;
        },
        { replace: true },
      );
    },
    [setParams],
  );

  const seen = useRef<string | null>(null);
  useEffect(() => {
    if (seen.current !== null && seen.current !== filters) setPage(1);
    seen.current = filters;
  }, [filters, setPage]);

  return [page, setPage];
}

/** The URL for another page of the current screen, other parameters kept. */
export function pageHref(search: string, page: number): string {
  const out = new URLSearchParams(search);
  if (page <= 1) out.delete("page");
  else out.set("page", String(page));
  const query = out.toString();
  return query === "" ? "?" : `?${query}`;
}
