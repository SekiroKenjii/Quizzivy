import { useMemo } from "react";
import { useTranslation } from "react-i18next";
import { Link, useLocation } from "react-router";
import { GraduationCap } from "lucide-react";
import { registerSkeleton } from "@/app/boot/handoff";
import { Button } from "@/components/ui/button";
import { rememberWorkspace } from "@/features/auth/accountPreferences";
import { useWorkspace } from "@/features/auth/permissions";
import { activeAdminNavId, adminNavFor } from "@/layouts/adminNav";
import { ConsoleShell } from "@/layouts/shell/ConsoleShell";
import { ConsoleSkeleton } from "@/layouts/TeacherSkeleton";
import { useAdminNavCounts } from "@/layouts/useAdminNavCounts";
import { useAuthStore } from "@/stores/auth";

registerSkeleton("admin", () => <ConsoleSkeleton workspace="admin" pageMax="1320px" />);

function TeacherView() {
  const { t } = useTranslation();
  return (
    <Button asChild variant="outline" className="h-8.5 gap-1.5 rounded-md px-3 text-sm">
      <Link
        to="/teacher"
        aria-label={t("adminShell.teacherView")}
        onClick={() => rememberWorkspace("teacher")}
      >
        <GraduationCap className="size-[0.9375rem]" aria-hidden="true" />
        <span className="hidden min-[768px]:inline">{t("adminShell.teacherView")}</span>
      </Link>
    </Button>
  );
}

/**
 * AdminLayout is the Admin console: the shared shell (`ConsoleShell`) with the
 * destinations the user's permissions open, the badge of the AI credit
 * requests waiting, and an outline button to the teacher view for a user who
 * has a teacher workspace. As the deck draws it, its top bar has no search,
 * bell or language switch.
 */
export default function AdminLayout() {
  const user = useAuthStore((s) => s.user);
  const { pathname } = useLocation();
  const counts = useAdminNavCounts();
  const teacher = useWorkspace("teacher");
  const groups = useMemo(() => adminNavFor(user), [user]);

  return (
    <ConsoleShell
      workspace="admin"
      groups={groups}
      activeId={activeAdminNavId(pathname)}
      counts={counts}
      end={teacher ? <TeacherView /> : null}
    />
  );
}
