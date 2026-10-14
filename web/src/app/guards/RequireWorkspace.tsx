import { Navigate, Outlet, useLocation } from "react-router";
import { consolePath, hasTeacherTwin } from "@/app/adminPaths";
import { LegacyTeacherRedirect } from "@/app/LegacyTeacherRedirect";
import { useWorkspace } from "@/features/auth/permissions";
import { homePathFor } from "@/features/auth/home";
import ForbiddenPage from "@/app/pages/ForbiddenPage";
import { useAuthStore } from "@/stores/auth";

/** The teacher's tree (§3), for anyone who may enter the teacher workspace. */
export function TeacherWorkspace() {
  const allowed = useWorkspace("teacher");
  if (!allowed) {
    return <ForbiddenPage />;
  }
  return <Outlet />;
}

/**
 * The Admin console's tree (§3), for anyone who may enter the admin
 * workspace, at an address of the console. Any other address beneath the old
 * prefix is a teacher address from before R4 and is sent to the same path under
 * the teacher's. A user without the workspace who opens an address of the
 * console gets the 403, except at the two that are also pages of the teacher's
 * tree (`hasTeacherTwin`), which a bookmark meant and which they are sent to.
 */
export function AdminWorkspace() {
  const allowed = useWorkspace("admin");
  const { pathname } = useLocation();
  const inConsole = consolePath(pathname);
  if (allowed && inConsole) {
    return <Outlet />;
  }
  if (!inConsole || hasTeacherTwin(pathname)) {
    return <LegacyTeacherRedirect />;
  }
  return <ForbiddenPage />;
}

/**
 * The student tree (§3), for anyone who may enter the student app. A staff
 * user without it is sent to their own console; anyone else gets the 403.
 */
export function StudentArea() {
  const student = useWorkspace("app");
  const teacher = useWorkspace("teacher");
  const admin = useWorkspace("admin");
  const home = useAuthStore((s) => homePathFor(s.user));
  if (student) {
    return <Outlet />;
  }
  if (teacher || admin) {
    return <Navigate to={home} replace />;
  }
  return <ForbiddenPage />;
}
