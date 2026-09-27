import { Navigate, Outlet, useLocation } from "react-router";
import { useAppState } from "@/stores/appState";
import { useAuthStore } from "@/stores/auth";

/** Where the forced password change lives. */
export const CHANGE_PASSWORD_PATH = "/change-password";

/**
 * Pathless guard: renders the tree only for a signed-in user.
 *
 * Renders nothing while the session is bootstrapping, under the boot splash,
 * rather than redirecting, or a
 * reload would bounce every user to /login before `GET /auth/me` answers.
 * Carries the attempted path as `?next=` so sign-in can return to it.
 */
export function RequireSession() {
  const location = useLocation();
  const isBootstrapping = useAuthStore((s) => s.isBootstrapping);
  const user = useAuthStore((s) => s.user);
  const bootError = useAppState((s) => (s.bootPhase === "failed" ? s.bootError : null));

  if (bootError) throw bootError;

  if (isBootstrapping) return null;

  if (!user) {
    const next = location.pathname + location.search;
    return <Navigate to={`/login?next=${encodeURIComponent(next)}`} replace />;
  }

  if (user.mustChangePassword && location.pathname !== CHANGE_PASSWORD_PATH) {
    return <Navigate to={CHANGE_PASSWORD_PATH} replace />;
  }

  return <Outlet />;
}
