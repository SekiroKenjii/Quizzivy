import { Navigate, Outlet, useLocation } from "react-router";
import { useTranslation } from "react-i18next";
import { useAuthStore } from "@/stores/auth";

/** Where the forced password change lives. */
export const CHANGE_PASSWORD_PATH = "/change-password";

/**
 * Pathless guard: renders the tree only for a signed-in user.
 *
 * Waits while the session is bootstrapping rather than redirecting, or a
 * reload would bounce every user to /login before `GET /auth/me` answers.
 * Carries the attempted path as `?next=` so sign-in can return to it.
 */
export function RequireSession() {
  const { t } = useTranslation();
  const location = useLocation();
  const isBootstrapping = useAuthStore((s) => s.isBootstrapping);
  const user = useAuthStore((s) => s.user);

  if (isBootstrapping) {
    return (
      <div
        className="text-muted-foreground flex min-h-svh items-center justify-center text-sm"
        role="status"
        aria-live="polite"
      >
        {t("common.loading")}
      </div>
    );
  }

  if (!user) {
    const next = location.pathname + location.search;
    return <Navigate to={`/login?next=${encodeURIComponent(next)}`} replace />;
  }

  if (user.mustChangePassword && location.pathname !== CHANGE_PASSWORD_PATH) {
    return <Navigate to={CHANGE_PASSWORD_PATH} replace />;
  }

  return <Outlet />;
}
