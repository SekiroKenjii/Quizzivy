import {
  AudioLines,
  ClipboardList,
  FileText,
  GraduationCap,
  LayoutDashboard,
  Library,
  SquarePen,
  Users,
} from "lucide-react";
import { holds, type Requirement } from "@/features/auth/permissions";
import type { NavDestination, NavGroup } from "@/layouts/shell/nav";
import type { components } from "@/lib/api/schema";

type NavUser = Pick<components["schemas"]["CurrentUser"], "permissions" | "workspaces">;

/**
 * TeacherNavItem is one destination of the teacher sidebar. `operation` is
 * the `operationId` of the list the destination opens and `requires` that
 * operation's permission.
 */
export interface TeacherNavItem extends NavDestination {
  operation: string;
  requires: Requirement;
}

/** TeacherNavGroup is a run of destinations under one label; the first has none. */
export interface TeacherNavGroup extends NavGroup {
  items: readonly TeacherNavItem[];
}

/**
 * TEACHER_NAV is the teacher sidebar in the deck's order. A module that has
 * not shipped is absent from it.
 */
export const TEACHER_NAV: readonly TeacherNavGroup[] = [
  {
    id: "home",
    items: [
      {
        id: "dashboard",
        to: "/teacher",
        label: "teacherShell.nav.dashboard",
        icon: LayoutDashboard,
        operation: "getDashboard",
        requires: "workspace.teacher",
      },
    ],
  },
  {
    id: "teaching",
    label: "teacherShell.group.teaching",
    items: [
      {
        id: "assignments",
        to: "/teacher/assignments",
        label: "teacherShell.nav.assignments",
        icon: ClipboardList,
        operation: "listAssignments",
        requires: "workspace.teacher",
        count: {
          figure: "liveAssignments",
          tone: "muted",
          sentence: "teacherShell.count.live",
        },
      },
      {
        id: "grading",
        to: "/teacher/grading",
        label: "teacherShell.nav.grading",
        icon: SquarePen,
        operation: "listAttempts",
        requires: ["teaching.grading", "teaching.attempts.intervene"],
        count: {
          figure: "toGrade",
          tone: "urgent",
          sentence: "teacherShell.count.toGrade",
        },
      },
      {
        id: "classes",
        to: "/teacher/classes",
        label: "teacherShell.nav.classes",
        icon: GraduationCap,
        operation: "listClasses",
        requires: "workspace.teacher",
      },
      {
        id: "students",
        to: "/teacher/students",
        label: "teacherShell.nav.students",
        icon: Users,
        operation: "listStudents",
        requires: "people.students.read",
      },
    ],
  },
  {
    id: "content",
    label: "teacherShell.group.content",
    items: [
      {
        id: "tests",
        to: "/teacher/tests",
        label: "teacherShell.nav.tests",
        icon: FileText,
        operation: "listTests",
        requires: "workspace.teacher",
      },
      {
        id: "bank",
        to: "/teacher/question-bank",
        label: "teacherShell.nav.questionBank",
        icon: Library,
        operation: "listQuestions",
        requires: "workspace.teacher",
      },
      {
        id: "media",
        to: "/teacher/media",
        label: "teacherShell.nav.media",
        icon: AudioLines,
        operation: "listMedia",
        requires: "workspace.teacher",
      },
    ],
  },
];

const ALIASES = [
  { prefix: "/teacher/imports", id: "tests" },
  { prefix: "/teacher/attempts", id: "assignments" },
] as const;

function normalised(pathname: string): string {
  let end = pathname.length;
  while (end > 1 && pathname[end - 1] === "/") end -= 1;
  return pathname.slice(0, end).toLowerCase();
}

function under(path: string, prefix: string): boolean {
  return path === prefix || path.startsWith(`${prefix}/`);
}

/**
 * reaches reports whether `user` holds the requirement of the destination
 * `id`, the rule the sidebar shows it by; an unknown id is never reached.
 */
export function reaches(user: NavUser | null | undefined, id: string): boolean {
  const item = TEACHER_NAV.flatMap((group) => group.items).find(
    (candidate) => candidate.id === id,
  );
  return item !== undefined && holds(user, item.requires);
}

/**
 * navFor returns the sidebar for `user`: the destinations whose requirement
 * the user holds, in order. A group left with no destination is dropped.
 */
export function navFor(user: NavUser | null | undefined): TeacherNavGroup[] {
  return TEACHER_NAV.map((group) => ({
    ...group,
    items: group.items.filter((item) => holds(user, item.requires)),
  })).filter((group) => group.items.length > 0);
}

/**
 * activeNavId returns the id of the destination `pathname` belongs to, or
 * null when it belongs to none, as Settings does. `/teacher` alone is the
 * dashboard; any other destination owns its own path and every path beneath
 * it. The import pages belong to Tests and an attempt's page to Assignments.
 */
export function activeNavId(pathname: string): string | null {
  const path = normalised(pathname);
  if (path === "/teacher") return "dashboard";
  const alias = ALIASES.find(({ prefix }) => under(path, prefix));
  if (alias) return alias.id;
  for (const group of TEACHER_NAV) {
    for (const item of group.items) {
      if (item.id !== "dashboard" && under(path, item.to)) return item.id;
    }
  }
  return null;
}
