import type { TFunction } from "i18next";
import {
  ClipboardList,
  FileText,
  Headphones,
  Library,
  Settings,
  User,
  type LucideIcon,
} from "lucide-react";
import type { Assignment } from "@/features/assignments/api";
import { statusAt } from "@/features/assignments/status";
import type { listQuestions } from "@/features/question-bank/api";
import type { Student } from "@/features/students/api";
import type { listTests } from "@/features/tests/api";
import { api } from "@/lib/api/client";
import { fold } from "@/lib/fold";
import { navFor, type TeacherNavItem } from "@/layouts/teacherNav";
import type { components } from "@/lib/api/schema";

/** PaletteGroup is a group of the palette's results, in the deck's order. */
export type PaletteGroup = "pages" | "assignments" | "students" | "tests" | "questions";

/** PALETTE_GROUPS lists the groups in the order the palette draws them. */
export const PALETTE_GROUPS: readonly PaletteGroup[] = [
  "pages",
  "assignments",
  "students",
  "tests",
  "questions",
];

/** PALETTE_LIMITS is how many results each searched group shows, as the deck draws them. */
export const PALETTE_LIMITS = {
  assignments: 4,
  students: 3,
  tests: 4,
  questions: 3,
} as const;

/** PaletteEntry is one row of the palette: what it says, its icon and where it goes. */
export interface PaletteEntry {
  id: string;
  group: PaletteGroup;
  label: string;
  hint: string;
  Icon: LucideIcon;
  to: string;
}

/** PalettePage is a destination the palette offers, with its label already folded for matching. */
export interface PalettePage {
  id: string;
  label: string;
  folded: string;
  Icon: LucideIcon;
  to: string;
}

const PAGE_ORDER = [
  "dashboard",
  "assignments",
  "grading",
  "tests",
  "bank",
  "students",
  "classes",
  "media",
] as const;

type TestRow = Awaited<ReturnType<typeof listTests>>["items"][number];
type QuestionRow = Awaited<ReturnType<typeof listQuestions>>["items"][number];

type NavUser = Pick<components["schemas"]["CurrentUser"], "permissions" | "workspaces">;

/**
 * palettePages lists the pages the palette offers to `user`, in the deck's
 * order: the sidebar's destinations the user may open, then Settings, which
 * every account has.
 */
export function palettePages(user: NavUser | null, t: TFunction): PalettePage[] {
  const byId = new Map<string, TeacherNavItem>();
  for (const group of navFor(user))
    for (const item of group.items) byId.set(item.id, item);
  const pages: PalettePage[] = [];
  for (const id of PAGE_ORDER) {
    const item = byId.get(id);
    if (item === undefined) continue;
    const label = t(item.label);
    pages.push({ id, label, folded: fold(label), Icon: item.icon, to: item.to });
  }
  const settings = t("nav.settings");
  pages.push({
    id: "settings",
    label: settings,
    folded: fold(settings),
    Icon: Settings,
    to: "/teacher/settings",
  });
  return pages;
}

/** pageEntries are the pages whose label holds `search`, accents and case aside. */
export function pageEntries(
  pages: readonly PalettePage[],
  search: string,
): PaletteEntry[] {
  const wanted = fold(search);
  return pages
    .filter((page) => wanted === "" || page.folded.includes(wanted))
    .map((page) => ({
      id: `page-${page.id}`,
      group: "pages",
      label: page.label,
      hint: "",
      Icon: page.Icon,
      to: page.to,
    }));
}

/** assignmentEntries name an assignment by its test, with its status as the hint. */
export function assignmentEntries(
  items: readonly Assignment[],
  now: Date,
  t: TFunction,
): PaletteEntry[] {
  return items.slice(0, PALETTE_LIMITS.assignments).map((assignment) => ({
    id: `assignment-${assignment.id}`,
    group: "assignments",
    label: assignment.testTitle,
    hint: t(`status.assignment.${statusAt(assignment, now)}`),
    Icon: ClipboardList,
    to: `/teacher/assignments/${assignment.id}`,
  }));
}

/** studentEntries open the students list searched for the student, with their first class as the hint. */
export function studentEntries(items: readonly Student[]): PaletteEntry[] {
  return items.slice(0, PALETTE_LIMITS.students).map((student) => ({
    id: `student-${student.id}`,
    group: "students",
    label: student.fullName,
    hint: student.classes[0]?.name ?? "",
    Icon: User,
    to: `/teacher/students?q=${encodeURIComponent(student.fullName)}`,
  }));
}

/** testEntries open a test, with its status as the hint. */
export function testEntries(items: readonly TestRow[], t: TFunction): PaletteEntry[] {
  return items.slice(0, PALETTE_LIMITS.tests).map((test) => ({
    id: `test-${test.id}`,
    group: "tests",
    label: test.title,
    hint: t(`status.test.${test.status}`),
    Icon: FileText,
    to: `/teacher/tests/${test.id}`,
  }));
}

/** questionEntries open a bank question, with its tags as the hint. */
export function questionEntries(items: readonly QuestionRow[]): PaletteEntry[] {
  return items.slice(0, PALETTE_LIMITS.questions).map((question) => ({
    id: `question-${question.id}`,
    group: "questions",
    label: question.prompt,
    hint: question.tags.join(", "),
    Icon: question.media?.kind === "audio" ? Headphones : Library,
    to: `/teacher/question-bank/${question.id}`,
  }));
}

/**
 * searchAssignments asks `listAssignments` for the newest assignments whose
 * test title matches `q`, accents aside, as the server folds it.
 */
export async function searchAssignments(q: string, signal: AbortSignal) {
  const query: Record<string, unknown> = { limit: PALETTE_LIMITS.assignments };
  if (q !== "") query["q"] = q;
  const page = await api("get", "/teacher/assignments", { query, signal });
  return page.items;
}
