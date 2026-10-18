import type { TFunction } from "i18next";
import type { DataColumn } from "@/components/shared/data/DataTable";
import type { Student } from "@/features/students/api";
import type { Locale } from "@/lib/i18n";
import { formatDateTime } from "@/lib/i18n/datetime";
import { AccountPill, ClassChips, Initials } from "./StudentCells";
import { averageOf, classLineOf, lastActiveOf } from "./studentText";

/**
 * studentColumns is the list as the deck draws it: Student (initials, name,
 * and a line of the email, then the classes and the last activity while
 * their columns are hidden), Classes from 780px of content width (two chips
 * and "+N", on one line), Average,
 * Last active from 920 and Account from 660.
 */
export function studentColumns(t: TFunction, locale: Locale): DataColumn<Student>[] {
  return [
    {
      id: "student",
      header: t("students.student"),
      track: "minmax(200px,2fr)",
      cell: (student, shown) => {
        const sub = [
          student.email,
          shown.has("classes") ? null : classLineOf(student, t),
          shown.has("last") ? null : lastActiveOf(student, locale, t),
        ]
          .filter((part) => part !== null)
          .join(" · ");
        return (
          <span className="flex min-w-0 items-center gap-2.5">
            <Initials name={student.fullName} size="row" />
            <span className="min-w-0">
              <span className="block truncate font-medium">{student.fullName}</span>
              <span className="text-muted-fg block truncate text-xs">{sub}</span>
            </span>
          </span>
        );
      },
    },
    {
      id: "classes",
      header: t("students.classes"),
      track: "minmax(150px,1.6fr)",
      showFrom: 780,
      cell: (student) => <ClassChips student={student} />,
    },
    {
      id: "average",
      header: t("students.average"),
      track: "80px",
      cell: (student) => (
        <span className="font-medium tabular-nums">{averageOf(student, t)}</span>
      ),
    },
    {
      id: "last",
      header: t("students.lastActive"),
      track: "110px",
      showFrom: 920,
      cell: (student) =>
        student.lastActiveAt && !student.stats.activity.live ? (
          <time
            dateTime={student.lastActiveAt}
            title={formatDateTime(student.lastActiveAt, locale)}
            className="text-muted-fg text-[13px] whitespace-nowrap"
          >
            {lastActiveOf(student, locale, t)}
          </time>
        ) : (
          <span className="text-muted-fg text-[13px] whitespace-nowrap">
            {lastActiveOf(student, locale, t)}
          </span>
        ),
    },
    {
      id: "account",
      header: t("students.accountHeader"),
      track: "130px",
      showFrom: 660,
      cell: (student) => <AccountPill student={student} />,
    },
  ];
}
