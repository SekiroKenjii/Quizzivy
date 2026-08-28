import { Navigate } from "react-router";
import { useTranslation } from "react-i18next";
import { homePathFor } from "@/features/auth/home";
import { useAuthStore } from "@/stores/auth";

/**
 * What "/" means, which depends on who is asking (§3).
 *
 * Waits for the session to settle first. A bare `<Navigate to="/login">` here
 * would send a signed-in user who typed the bare domain to the sign-in form,
 * and would race the bootstrap on every cold load of "/".
 */
export function HomeRedirect() {
  const { t } = useTranslation();
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
  return <Navigate to={user ? homePathFor(user) : "/login"} replace />;
}
