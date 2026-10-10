import { useTranslation } from "react-i18next";
import { Eye, SquarePen } from "lucide-react";
import type { AssignmentDraft } from "@/features/assignments/draft";
import { previewRules } from "@/features/assignments/studentRules";
import { windowDays } from "@/features/assignments/wizardValues";
import type { Locale } from "@/lib/i18n";

/**
 * WizardAside is the deck's column beside the wizard: the summary lines,
 * what students will read on the Test intro (the same sentences, from
 * `previewRules`), and how much grading by hand the assignment will make.
 * `students` is the roster's count, or null while it is being counted.
 */
export function WizardAside({
  draft,
  students,
}: Readonly<{ draft: AssignmentDraft; students: number | null }>) {
  const { t, i18n } = useTranslation();
  const manual = draft.picked?.version.manualCount ?? 0;
  const rules = previewRules(draft, t, i18n.language as Locale, new Date());
  const lines: [string, string][] = [
    [
      t("assignments.wizard.aside.students"),
      students === null ? "—" : String(students),
    ],
    [
      t("assignments.wizard.aside.window"),
      t("assignments.wizard.aside.days", { count: windowDays(draft) }),
    ],
    [
      t("assignments.wizard.aside.timeLimit"),
      t("assignments.wizard.minutes", { count: draft.durationMinutes }),
    ],
    [t("assignments.wizard.aside.attempts"), String(draft.maxAttempts)],
    [
      t("assignments.wizard.aside.manual"),
      t("assignments.wizard.aside.perAttempt", { count: manual }),
    ],
  ];

  return (
    <aside
      aria-label={t("assignments.wizard.aside.label")}
      className="sticky top-0 flex min-w-0 flex-[2_1_280px] flex-col gap-3 self-start"
    >
      <div className="bg-card shadow-card border-border rounded-xl border p-4">
        <dl className="border-border m-0 mb-3.5 flex flex-col gap-2 border-b pb-3.5">
          {lines.map(([label, value]) => (
            <div key={label} className="flex justify-between gap-3 text-sm">
              <dt className="text-muted-fg">{label}</dt>
              <dd className="m-0 text-right font-medium tabular-nums">{value}</dd>
            </div>
          ))}
        </dl>
        <h3 className="text-muted-fg text-meta mb-2.5 flex items-center gap-1.5 font-medium">
          <Eye aria-hidden="true" className="size-3.5" />
          {t("assignments.wizard.aside.reads")}
        </h3>
        <ul className="m-0 flex list-none flex-col gap-2 p-0 text-base leading-[1.55]">
          {rules.map((rule) => (
            <li key={rule.id} className="flex gap-2">
              <span aria-hidden="true" className="text-muted-fg">
                ·
              </span>
              <span>{rule.text}</span>
            </li>
          ))}
        </ul>
      </div>
      {draft.picked !== null && (
        <p className="bg-warning-soft text-warning-ink m-0 flex gap-2.5 rounded-xl px-4 py-3.5 text-sm leading-normal">
          <SquarePen aria-hidden="true" className="mt-px size-4 flex-none" />
          <span>
            {manual > 0
              ? t("assignments.wizard.aside.estimate", {
                  total: manual * (students ?? 0),
                  questions: t("assignments.wizard.questions", { count: manual }),
                  students: t("assignments.wizard.classStudents", {
                    count: students ?? 0,
                  }),
                })
              : t("assignments.wizard.aside.auto")}
          </span>
        </p>
      )}
    </aside>
  );
}
