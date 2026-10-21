import { useTranslation } from "react-i18next";
import { initials } from "@/components/ui/avatar";
import type { Student } from "@/features/students/api";
import type { Locale } from "@/lib/i18n";
import { cn } from "@/lib/utils";
import {
  accountOf,
  averageOf,
  classLineOf,
  lastActiveOf,
  type StudentAccount,
} from "./studentText";

const ACCOUNT: Record<StudentAccount, { pill: string; dot: string }> = {
  active: { pill: "bg-success-soft text-success-ink", dot: "bg-success" },
  notSignedIn: { pill: "bg-muted text-muted-fg", dot: "bg-muted-fg" },
  reset: { pill: "bg-warning-soft text-warning-ink", dot: "bg-warning" },
  suspended: { pill: "bg-danger-soft text-danger-ink", dot: "bg-danger" },
};

/** AccountPill is the deck's account status: Active, Not signed in, Password reset or Suspended. */
export function AccountPill({ student }: Readonly<{ student: Student }>) {
  const { t } = useTranslation();
  const account = accountOf(student);
  const tone = ACCOUNT[account];
  return (
    <span
      className={cn(
        "inline-flex h-5.5 items-center gap-1.5 rounded-full px-2 text-xs font-medium whitespace-nowrap",
        tone.pill,
      )}
    >
      <span
        aria-hidden="true"
        className={cn("size-1.5 flex-none rounded-full", tone.dot)}
      />
      {t(`students.account.${account}`)}
    </span>
  );
}

const SHOWN_CLASSES = 2;

/**
 * ClassChips is a student's classes in the Classes column, on one line that
 * never leaves its cell: the first two as chips that truncate, each with its
 * full name as a title, then "+N" for the rest, which names them in its
 * title and to a screen reader. A student in no class reads "No class yet".
 */
export function ClassChips({ student }: Readonly<{ student: Student }>) {
  const { t } = useTranslation();
  if (student.classes.length === 0)
    return (
      <span className="text-muted-fg text-[13px]">{t("students.noClassYet")}</span>
    );
  const shown = student.classes.slice(0, SHOWN_CLASSES);
  const rest = student.classes.slice(SHOWN_CLASSES);
  const restNames = rest.map((klass) => klass.name).join(", ");
  return (
    <span className="flex min-w-0 flex-nowrap items-center gap-1.25 overflow-hidden">
      {shown.map((klass) => (
        <span
          key={klass.id}
          title={klass.name}
          className="min-w-0 truncate rounded-md border px-1.75 py-px text-xs whitespace-nowrap"
        >
          {klass.name}
        </span>
      ))}
      {rest.length === 0 ? null : (
        <span
          title={restNames}
          className="flex-none rounded-md border px-1.75 py-px text-xs whitespace-nowrap tabular-nums"
        >
          <span aria-hidden="true">{t("students.moreCount", { n: rest.length })}</span>
          <span className="sr-only">
            {t("students.moreClasses", { count: rest.length, names: restNames })}
          </span>
        </span>
      )}
    </span>
  );
}

/** Initials is a student's initials in the deck's muted circle, 32px in a row and 36 on a card. */
export function Initials({
  name,
  size,
}: Readonly<{ name: string; size: "row" | "card" }>) {
  return (
    <span
      aria-hidden="true"
      className={cn(
        "bg-muted grid flex-none place-items-center rounded-full font-semibold",
        size === "row" ? "size-8 text-[11.5px]" : "size-9 text-xs",
      )}
    >
      {initials(name)}
    </span>
  );
}

/** StudentCard is a student on a phone: initials, name, classes and last activity, average. */
export function StudentCard({
  student,
  locale,
}: Readonly<{ student: Student; locale: Locale }>) {
  const { t } = useTranslation();
  return (
    <span className="flex w-full min-w-0 items-center gap-3">
      <Initials name={student.fullName} size="card" />
      <span className="min-w-0 flex-1">
        <span className="block font-medium">{student.fullName}</span>
        <span className="text-muted-fg block truncate text-[12.5px]">
          {classLineOf(student, t)} · {lastActiveOf(student, locale, t)}
        </span>
      </span>
      <span className="font-semibold tabular-nums">{averageOf(student, t)}</span>
    </span>
  );
}
