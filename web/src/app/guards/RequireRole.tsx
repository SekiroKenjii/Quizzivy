import { lazy, Suspense } from "react";
import { Navigate, Outlet } from "react-router";
import { useAuthStore } from "@/stores/auth";

const ForbiddenPage = lazy(() => import("@/app/pages/ForbiddenPage"));

/**
 * The teacher's tree (§3).
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
    return (
      <Suspense fallback={null}>
        <ForbiddenPage />
      </Suspense>
    );
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
