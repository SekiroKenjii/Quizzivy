import { Navigate, Outlet } from "react-router";
import { useWorkspace } from "@/features/auth/permissions";
import ForbiddenPage from "@/app/pages/ForbiddenPage";

/** The teacher's tree (§3), for anyone who may enter the teacher workspace. */
export function TeacherWorkspace() {
  const allowed = useWorkspace("teacher");
  if (!allowed) {
    return <ForbiddenPage />;
  }
  return <Outlet />;
}

/**
 * The student tree (§3), for anyone who may enter the student app. A staff
 * user without it is sent to the teacher's tree; anyone else gets the 403.
 */
export function StudentArea() {
  const student = useWorkspace("app");
  const teacher = useWorkspace("teacher");
  const admin = useWorkspace("admin");
  if (student) {
    return <Outlet />;
  }
  if (teacher || admin) {
    return <Navigate to="/admin" replace />;
  }
  return <ForbiddenPage />;
}
