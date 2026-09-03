import { Navigate, Outlet } from "react-router";
import { useAuthStore } from "@/stores/auth";
import ForbiddenPage from "@/app/pages/ForbiddenPage";

/** The teacher's tree (§3). */
export function AdminOnly() {
  const role = useAuthStore((s) => s.user?.role);
  if (role !== "admin") {
    return <ForbiddenPage />;
  }
  return <Outlet />;
}

/** The student tree (§3). */
export function StudentArea() {
  const role = useAuthStore((s) => s.user?.role);
  if (role === "admin") {
    return <Navigate to="/admin" replace />;
  }
  return <Outlet />;
}
