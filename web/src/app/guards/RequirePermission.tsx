import { Outlet, useMatches } from "react-router";
import ForbiddenPage from "@/app/pages/ForbiddenPage";
import { holds } from "@/features/auth/permissions";
import { adminHandleOf } from "@/app/guards/adminHandle";
import { useAuthStore } from "@/stores/auth";

/**
 * RequirePermission is the Admin console's per-route gate: it reads the
 * `permission` of the leaf route's handle and renders the 403 page unless the
 * signed-in user holds it, so a custom role that holds only
 * `system.audit.read` gets a 403 at the users page and not an empty screen.
 * A route that declares no permission is refused too. It sits above the
 * console's layout, which a 403 therefore replaces.
 */
export function RequirePermission() {
  const handle = adminHandleOf(useMatches().at(-1)?.handle);
  const allowed = useAuthStore(
    (s) => handle !== null && holds(s.user, handle.permission),
  );
  return allowed ? <Outlet /> : <ForbiddenPage />;
}
