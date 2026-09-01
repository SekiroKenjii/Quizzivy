import { Navigate, Outlet } from "react-router";
import { useAuthStore } from "@/stores/auth";
import ForbiddenPage from "@/app/pages/ForbiddenPage";

/**
 * The teacher's tree (§3).
 *
 * Imported eagerly, unlike most pages. Everything heavy it renders --
 * AuthLayout, the drawings, the brand kit -- is already in the eager graph via
 * ErrorBoundary, so splitting it saved almost nothing and cost a blank screen:
 * `Suspense fallback={null}` while a chunk loads, at the exact moment a student
 * is trying to work out why a page will not open.
 *
 * A student here gets a 403 PAGE, not a redirect. §5.4 is explicit and the
 * reason is diagnosis: a redirect makes a permissions mistake look like a
 * navigation quirk, and the person who has to work out why the link "does
 * nothing" is the teacher. The server refuses these routes too
 * (httpx.RequireRole) -- this is the same answer, rendered.
 */
export function AdminOnly() {
  const role = useAuthStore((s) => s.user?.role);
  if (role !== "admin") {
    return <ForbiddenPage />;
  }
  return <Outlet />;
}

/**
 * The student tree (§3).
 *
 * An admin here IS redirected, and the asymmetry with AdminOnly is deliberate:
 * a teacher landing on /app has followed a stale link or a bookmark, and /admin
 * is unambiguously where they meant to be. There is no misconfiguration to
 * surface -- they have more access, not less.
 */
export function StudentArea() {
  const role = useAuthStore((s) => s.user?.role);
  if (role === "admin") {
    return <Navigate to="/admin" replace />;
  }
  return <Outlet />;
}
