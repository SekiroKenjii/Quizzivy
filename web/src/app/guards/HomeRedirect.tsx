import { Navigate } from "react-router";
import { homePathFor } from "@/features/auth/home";
import { useAppState } from "@/stores/appState";
import { useAuthStore } from "@/stores/auth";

/**
 * What "/" means, which depends on who is asking (§3).
 *
 * Waits for the session to settle first, rendering nothing under the boot
 * splash. A bare `<Navigate to="/login">` here
 * would send a signed-in user who typed the bare domain to the sign-in form,
 * and would race the bootstrap on every cold load of "/".
 */
export function HomeRedirect() {
  const isBootstrapping = useAuthStore((s) => s.isBootstrapping);
  const user = useAuthStore((s) => s.user);
  const bootError = useAppState((s) => (s.bootPhase === "failed" ? s.bootError : null));

  if (bootError) throw bootError;

  if (isBootstrapping) return null;
  return <Navigate to={user ? homePathFor(user) : "/login"} replace />;
}
