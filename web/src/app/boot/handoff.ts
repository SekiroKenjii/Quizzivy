import type { ReactNode } from "react";
import { teacherHandleOf } from "@/layouts/shell/handle";

/** Shell is the console a route belongs to. */
export type Shell = "teacher" | "admin" | "student";

const skeletons = new Map<Shell, () => ReactNode>();

/**
 * registerSkeleton gives a shell the skeleton the splash fades onto, drawn
 * beneath it while it leaves. R1 registers none, so the splash fades straight
 * onto the page; each rebuilt console registers its own.
 */
export function registerSkeleton(shell: Shell, skeleton: () => ReactNode): void {
  skeletons.set(shell, skeleton);
}

/**
 * skeletonFor draws the skeleton registered for the shell at pathname, if
 * any. The teacher's is drawn only when `handle`, the leaf route's, is a
 * teacher handle: a page not yet rebuilt renders in the old shell, which the
 * frame does not resemble.
 */
export function skeletonFor(pathname: string, handle?: unknown): ReactNode {
  const shell = shellFor(pathname);
  if (shell === null) return null;
  if (shell === "teacher" && teacherHandleOf(handle) === null) return null;
  const skeleton = skeletons.get(shell);
  return skeleton === undefined ? null : skeleton();
}

function shellFor(pathname: string): Shell | null {
  if (pathname === "/teacher" || pathname.startsWith("/teacher/")) return "teacher";
  if (pathname.startsWith("/app/attempts/") && !pathname.endsWith("/result"))
    return null;
  if (pathname === "/app" || pathname.startsWith("/app/")) return "student";
  return null;
}
