import { useEffect, useRef } from "react";
import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { ArrowRight, Check } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useLocale } from "@/lib/i18n/useLocale";
import { nfc } from "@/lib/nfc";
import type { ImportReviewSummary } from "../../api";

/**
 * CommittedCard says the import's draft test exists: its title, what it
 * holds when the review is still at hand (`summary`), and the way on to the
 * test builder or back to the imports. It is a status, so a screen reader
 * hears it when it replaces the summary; `focus` moves focus to its heading
 * when it appears, for the draft the teacher has just created.
 */
export function CommittedCard({
  testId,
  title,
  summary,
  focus = false,
}: Readonly<{
  testId: string;
  title: string;
  summary: ImportReviewSummary | null;
  focus?: boolean;
}>) {
  const { t } = useTranslation();
  const locale = useLocale();
  const heading = useRef<HTMLHeadingElement>(null);
  useEffect(() => {
    if (focus) heading.current?.focus();
  }, [focus]);
  return (
    <section
      role="status"
      className="bg-card shadow-card flex flex-col gap-3 rounded-xl border p-4.5"
    >
      <span className="bg-success-soft text-success-ink grid size-10.5 place-items-center rounded-[11px]">
        <Check aria-hidden="true" className="size-5" />
      </span>
      <div>
        <h2
          ref={heading}
          tabIndex={-1}
          className="text-title m-0 font-semibold [overflow-wrap:anywhere] outline-none"
        >
          {t("imports.confirm.doneTitle", { title: nfc(title) })}
        </h2>
        <p className="text-muted-fg m-0 mt-0.5 text-sm leading-normal">
          {summary === null
            ? t("imports.confirm.doneBodyPlain")
            : t("imports.confirm.doneBody", {
                questions: t("imports.review.questionsCount", {
                  count: summary.included,
                }),
                points: new Intl.NumberFormat(locale).format(
                  Number(summary.totalPoints),
                ),
              })}
        </p>
      </div>
      <Button asChild className="h-10 rounded-[9px] text-base font-semibold">
        <Link to={`/teacher/tests/${testId}/edit`}>
          {t("imports.confirm.openBuilder")}
          <ArrowRight aria-hidden="true" />
        </Link>
      </Button>
      <Button asChild variant="outline" className="text-ui h-10 rounded-[9px]">
        <Link to="/teacher/imports">{t("imports.backToHistory")}</Link>
      </Button>
    </section>
  );
}
