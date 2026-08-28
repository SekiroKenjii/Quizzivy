import { Navigate, Outlet, useLocation } from "react-router";
import { useTranslation } from "react-i18next";
import { useAuthStore } from "@/stores/auth";

/** Where the forced password change lives. */
export const CHANGE_PASSWORD_PATH = "/change-password";

/**
 * Everything behind a session (§5.4).
 *
 * Three states, in this order, and the order matters:
 *
 *  1. Still bootstrapping -> WAIT. Redirecting here would flash /login on every
 *     reload and throw away the deep link the user actually followed.
 *  2. No session -> /login?next=<path>, so signing in returns them to where
 *     they were going rather than to a generic home.
 *  3. `mustChangePassword` -> /change-password, from every route.
 *
 * A Google-only account never reaches (3): `must_change_password` requires a
 * password at the database level (D-16's CHECK), so the flag cannot be true
 * for an account that has none.
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
