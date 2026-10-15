import { useMemo } from "react";
import { useLocation } from "react-router";
import { registerSkeleton } from "@/app/boot/handoff";
import { modules } from "@/app/modules";
import { NotificationBell } from "@/features/notifications/components/NotificationBell";
import { CommandPalette } from "@/features/search/CommandPalette";
import { useCommandPalette } from "@/features/search/useCommandPalette";
import { ConsoleShell } from "@/layouts/shell/ConsoleShell";
import { useNavCounts } from "@/layouts/shell/useNavCounts";
import { activeNavId, navFor, reaches } from "@/layouts/teacherNav";
import { TeacherSkeleton } from "@/layouts/TeacherSkeleton";
import { useAuthStore } from "@/stores/auth";

registerSkeleton("teacher", () => <TeacherSkeleton />);

/**
 * TeacherLayout is the teacher console: the shared shell (`ConsoleShell`)
 * with the teacher's destinations, their badges, the command palette's search
 * button and the notification bell.
 */
export default function TeacherLayout() {
  const user = useAuthStore((s) => s.user);
  const { pathname } = useLocation();
  const counts = useNavCounts();
  const palette = useCommandPalette();
  const groups = useMemo(() => navFor(user), [user]);

  return (
    <>
      <ConsoleShell
        workspace="teacher"
        groups={groups}
        activeId={activeNavId(pathname)}
        counts={counts}
        onSearch={() => palette.setOpen(true)}
        end={
          modules.notifications ? (
            <NotificationBell
              audience="teacher"
              unread={counts?.unread ?? 0}
              canGrade={reaches(user, "grading")}
            />
          ) : null
        }
      />
      <CommandPalette open={palette.open} onOpenChange={palette.setOpen} />
    </>
  );
}
