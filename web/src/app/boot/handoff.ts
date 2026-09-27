import type { ReactNode } from "react";

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

/** skeletonFor draws the skeleton registered for the shell at pathname, if any. */
export function skeletonFor(pathname: string): ReactNode {
  const shell = shellFor(pathname);
  const skeleton = shell === null ? undefined : skeletons.get(shell);
  return skeleton === undefined ? null : skeleton();
}

function shellFor(pathname: string): Shell | null {
  if (pathname === "/admin" || pathname.startsWith("/admin/")) return "teacher";
  if (pathname.startsWith("/app/attempts/") && !pathname.endsWith("/result"))
    return null;
  if (pathname === "/app" || pathname.startsWith("/app/")) return "student";
  return null;
}
