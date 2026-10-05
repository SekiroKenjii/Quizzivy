import { useCallback, useEffect, useRef } from "react";
import { useSearchParams } from "react-router";
import { parsePage, parsePageSize } from "@/lib/pagination";

/**
 * PAGE_SIZES is the rows per page a list offers, as the design deck draws
 * them. The first is the default, and the absence of `?size=`.
 */
export const PAGE_SIZES = [10, 20, 30, 50] as const;

/**
 * The current page, kept in the URL (`?page=3`) so it survives a reload and
 * can be shared. Every other search parameter is preserved; page 1 is the
 * absence of the parameter, so the first page's URL is the plain route.
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
    if (seen.current !== null && seen.current !== filters && page > 1) setPage(1);
    seen.current = filters;
  }, [filters, page, setPage]);

  return [page, setPage];
}

/**
 * usePageSize keeps a list's rows per page in the URL (`?size=20`), beside
 * `usePage`'s page. A value that is not one of `sizes` reads as the first of
 * them, which is the default and the absence of the parameter. The setter
 * replaces the history entry, goes back to page 1 by removing `page` in the
 * same update, and keeps every other search parameter. A screen sends the
 * size to the API as `limit` and puts it in its query key.
 */
export function usePageSize(
  sizes: readonly number[] = PAGE_SIZES,
): [number, (size: number) => void] {
  const [params, setParams] = useSearchParams();
  const size = parsePageSize(params.get("size"), sizes);
  const first = sizes[0];
  const setSize = useCallback(
    (next: number) => {
      setParams(
        (current) => {
          const out = new URLSearchParams(current);
          out.delete("page");
          if (next === first) out.delete("size");
          else out.set("size", String(next));
          return out;
        },
        { replace: true },
      );
    },
    [setParams, first],
  );
  return [size, setSize];
}

/** The URL for another page of the current screen, other parameters kept. */
export function pageHref(search: string, page: number): string {
  const out = new URLSearchParams(search);
  if (page <= 1) out.delete("page");
  else out.set("page", String(page));
  const query = out.toString();
  return query === "" ? "?" : `?${query}`;
}
