import { useMatches } from "react-router";
import AdminLayout from "@/layouts/AdminLayout";
import { teacherHandleOf } from "@/layouts/shell/handle";
import TeacherLayout from "@/layouts/TeacherLayout";

/**
 * TeacherShell picks the shell of a page under `/teacher`: TeacherLayout
 * when the leaf route declares a teacher handle, and the old AdminLayout,
 * forced light and with its slots, for a page not yet rebuilt. A handle on a
 * parent route moves nothing.
 */
export default function TeacherShell() {
  const leaf = useMatches().at(-1);
  return teacherHandleOf(leaf?.handle) === null ? <AdminLayout /> : <TeacherLayout />;
}
