import { useCallback, useEffect, useRef } from "react";
import { useLocation, useNavigate, useSearchParams } from "react-router";
import { parsePage, parsePageSize } from "@/lib/pagination";

/**
 * PAGE_SIZES is the rows per page a list offers, as the design deck draws
 * them. The first is the default, and the absence of `?size=`.
 */
export const PAGE_SIZES = [10, 20, 30, 50] as const;

/** usePage keeps the page in search parameters and optionally preserves the hash during setters and filter resets. */
export function usePage(
  filters = "",
  preserveHash = false,
): [number, (page: number) => void] {
  const [params, setParams] = useSearchParams();
  const { pathname, search, hash } = useLocation();
  const navigate = useNavigate();
  const page = parsePage(params.get("page"));
  const setPage = useCallback(
    (next: number) => {
      const update = (current: URLSearchParams) => {
        const out = new URLSearchParams(current);
        if (next <= 1) out.delete("page");
        else out.set("page", String(next));
        return out;
      };
      if (preserveHash)
        void navigate(
          { pathname, search: `?${update(new URLSearchParams(search))}`, hash },
          { replace: true },
        );
      else setParams(update, { replace: true });
    },
    [setParams, pathname, search, hash, navigate, preserveHash],
  );

  const seen = useRef<string | null>(null);
  useEffect(() => {
    if (seen.current !== null && seen.current !== filters && page > 1) setPage(1);
    seen.current = filters;
  }, [filters, page, setPage]);

  return [page, setPage];
}

/** usePageSize keeps an offered size in search parameters, resets the page and optionally preserves the hash. */
export function usePageSize(
  sizes: readonly number[] = PAGE_SIZES,
  preserveHash = false,
): [number, (size: number) => void] {
  const [params, setParams] = useSearchParams();
  const { pathname, search, hash } = useLocation();
  const navigate = useNavigate();
  const size = parsePageSize(params.get("size"), sizes);
  const first = sizes[0];
  const setSize = useCallback(
    (next: number) => {
      const update = (current: URLSearchParams) => {
        const out = new URLSearchParams(current);
        out.delete("page");
        if (next === first) out.delete("size");
        else out.set("size", String(next));
        return out;
      };
      if (preserveHash)
        void navigate(
          { pathname, search: `?${update(new URLSearchParams(search))}`, hash },
          { replace: true },
        );
      else setParams(update, { replace: true });
    },
    [setParams, first, pathname, search, hash, navigate, preserveHash],
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
