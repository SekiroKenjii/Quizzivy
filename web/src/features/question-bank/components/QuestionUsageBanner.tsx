import { useTranslation } from "react-i18next";
import { Link } from "react-router";
import { FileText } from "lucide-react";
import type { AdminQuestion } from "@/features/question-bank/api";

const SHOWN = 3;

/**
 * QuestionUsageBanner is the muted line above a saved question in the
 * Question editor: "Used in {n} tests. Saving changes future tests only." or
 * "Not used in any test yet.", then up to three of those tests as chips that
 * open their builder. It counts the draft outlines that reference the
 * question; published versions keep their own copy.
 */
export function QuestionUsageBanner({
  question,
}: Readonly<{ question: Pick<AdminQuestion, "usedInTests" | "usedIn"> }>) {
  const { t } = useTranslation();
  const count = question.usedInTests ?? question.usedIn?.length ?? 0;
  const tests = (question.usedIn ?? []).slice(0, SHOWN);
  return (
    <div className="bg-muted flex flex-wrap items-center gap-2.5 rounded-[10px] px-3.5 py-2.5 text-[13px]">
      <FileText aria-hidden="true" className="text-muted-fg size-3.75 shrink-0" />
      <span className="min-w-0 flex-[1_1_260px]">
        {count > 0 ? t("questionEditor.usedIn", { count }) : t("questionEditor.unused")}
      </span>
      {tests.map((test) => (
        <Link
          key={test.id}
          to={`/teacher/tests/${test.id}/edit`}
          className="bg-card hover:bg-hover inline-flex h-6.5 max-w-full items-center truncate rounded-full border px-2.25 text-xs font-medium"
        >
          <span className="truncate">{test.title}</span>
        </Link>
      ))}
    </div>
  );
}
