import { Navigate, Outlet, useLocation } from "react-router";
import { consolePath } from "@/app/adminPaths";
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
 * prefix, and every one for a user without the workspace, is a teacher address
 * from before R4 and is sent to the same path under the teacher's.
 */
export function AdminWorkspace() {
  const allowed = useWorkspace("admin");
  const { pathname } = useLocation();
  if (!allowed || !consolePath(pathname)) {
    return <LegacyTeacherRedirect />;
  }
  return <Outlet />;
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
