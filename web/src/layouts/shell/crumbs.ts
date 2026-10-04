import { createContext, useContext, useLayoutEffect } from "react";
import type { TeacherCrumb } from "@/layouts/shell/handle";

/**
 * PageCrumb is one breadcrumb as the top bar shows it: the text, and the
 * address it links to when it is not the page itself.
 */
export interface PageCrumb {
  label: string;
  to?: string;
}

/**
 * CrumbTailContext carries the teacher shell's setter for the tail a page
 * names with useCrumbs. It is null outside the shell.
 */
export const CrumbTailContext = createContext<
  ((tail: readonly PageCrumb[] | null) => void) | null
>(null);

/**
 * trailOf is the breadcrumb trail of a route: each crumb of its handle with
 * its key translated by `t`, and the last crumbs replaced by `tail`, entry
 * for entry. A replaced crumb takes the tail's label, and its `to` when the
 * tail gives one. The trail keeps the handle's length: a tail longer than
 * the trail gives its last entries.
 */
export function trailOf(
  crumb: readonly TeacherCrumb[],
  tail: readonly PageCrumb[] | null | undefined,
  t: (key: string) => string,
): PageCrumb[] {
  const named = (tail ?? []).slice(-crumb.length);
  const kept = crumb.length - named.length;
  return crumb.map((entry, index) => {
    const own = index < kept ? undefined : named[index - kept];
    const to = own?.to ?? entry.to;
    const label = own?.label ?? t(entry.key);
    return to === undefined ? { label } : { label, to };
  });
}

function encode(tail: readonly PageCrumb[] | null | undefined): string {
  if (!tail || tail.length === 0) return "";
  return JSON.stringify(tail.map(({ label, to }) => [label, to ?? null]));
}

function decode(signature: string): PageCrumb[] | null {
  if (signature === "") return null;
  const entries = JSON.parse(signature) as [string, string | null][];
  return entries.map(([label, to]) => (to === null ? { label } : { label, to }));
}

/**
 * useCrumbs names the record a page shows in the teacher shell's breadcrumb
 * trail and document title. While the page is mounted, a tail of n crumbs
 * replaces the last n crumbs of its route's trail; null, undefined or an
 * empty tail leaves the route's own labels, which is what shows while a
 * record loads. The tail is compared by value, so a fresh array on every
 * render is fine. Outside the shell it does nothing.
 */
export function useCrumbs(tail: readonly PageCrumb[] | null | undefined): void {
  const set = useContext(CrumbTailContext);
  const signature = encode(tail);
  useLayoutEffect(() => {
    if (set === null) return;
    set(decode(signature));
    return () => set(null);
  }, [set, signature]);
}
