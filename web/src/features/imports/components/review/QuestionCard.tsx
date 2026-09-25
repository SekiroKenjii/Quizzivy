import { memo } from "react";
import { useTranslation } from "react-i18next";
import type { TFunction } from "i18next";
import { Check, CircleAlert, Pencil, TriangleAlert } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { ContentView } from "@/components/shared/content/ContentView";
import { cn } from "@/lib/utils";
import type { ImportDraftQuestion, ImportFinding } from "../../api";
import { isChoiceType } from "../../draft";
import { isUnresolved } from "../../findings";

function missingAnswerText(question: ImportDraftQuestion, t: TFunction): string {
  if (question.type === "short_answer") return t("imports.review.noSample");
  return question.origins.answer === "teacher_entered"
    ? t("imports.review.answerNotSet")
    : t("imports.review.answerMissing");
}

/** AnswerSummary states a question's key in words: known, missing from the document, or contradicted by it. */
export function AnswerSummary({
  question,
}: Readonly<{ question: ImportDraftQuestion }>) {
  const { t } = useTranslation();
  if (question.answer.state === "conflict")
    return <p className="text-sm">{t("imports.review.answerConflict")}</p>;
  if (question.answer.state === "unknown")
    return (
      <p className="text-muted-foreground text-sm">{missingAnswerText(question, t)}</p>
    );
  if (question.type === "short_answer")
    return (
      <p className="text-sm">
        {t("imports.review.sampleIs", { text: question.answer.text ?? "" })}
      </p>
    );
  if (question.type === "fill_blank")
    return (
      <ul className="space-y-0.5 text-sm">
        {question.blanks.map((blank, index) => (
          <li key={blank.gapId}>
            {t("imports.review.blankAnswers", {
              label: blank.label ?? String(index + 1),
              answers: blank.accepted.join(" / "),
            })}
          </li>
        ))}
      </ul>
    );
  return null;
}

/** QuestionCard is the read-only view of a question that is not selected; it mounts no editor. */
export const QuestionCard = memo(function QuestionCard({
  question,
  findings,
  onSelect,
}: Readonly<{
  question: ImportDraftQuestion;
  findings: readonly ImportFinding[];
  onSelect: (questionId: string) => void;
}>) {
  const { t } = useTranslation();
  const blocking = findings.filter((finding) => finding.severity === "blocking").length;
  const review = findings.filter(
    (finding) => finding.severity === "review_required" && isUnresolved(finding),
  ).length;
  const edited = Object.values(question.origins).includes("teacher_entered");
  const keys = new Set(question.answer.optionIds);
  const excluded = question.excluded !== undefined;
  return (
    <article
      data-question-id={question.id}
      aria-label={t("imports.review.questionLabel", { label: question.label })}
      className={cn("space-y-2.5 rounded-lg border p-4", excluded && "bg-muted/40")}
    >
      <header className="flex flex-wrap items-center gap-2">
        <button
          type="button"
          onClick={() => onSelect(question.id)}
          className="focus-visible:ring-ring rounded-sm text-sm font-medium hover:underline focus-visible:ring-2 focus-visible:outline-none"
        >
          {t("imports.review.questionLabel", { label: question.label })}
        </button>
        <span className="text-muted-foreground text-xs">
          {t(`imports.type.${question.type}`)}
        </span>
        <span className="text-muted-foreground text-xs tabular-nums">
          {t("imports.review.pointsShort", { points: question.points })}
        </span>
        {excluded ? (
          <Badge variant="outline">{t("imports.review.excludedBadge")}</Badge>
        ) : null}
        {edited ? (
          <Badge variant="secondary">
            <Pencil aria-hidden="true" />
            {t("imports.origin.teacher_entered")}
          </Badge>
        ) : null}
        <span className="ml-auto flex items-center gap-1.5">
          {blocking > 0 ? (
            <Badge variant="danger">
              <CircleAlert aria-hidden="true" />
              {t("imports.review.blockingCount", { count: blocking })}
            </Badge>
          ) : null}
          {review > 0 ? (
            <Badge variant="warning">
              <TriangleAlert aria-hidden="true" />
              {t("imports.review.reviewCount", { count: review })}
            </Badge>
          ) : null}
        </span>
      </header>
      {excluded ? (
        <p className="text-muted-foreground text-xs">
          {t("imports.review.excludedBecause", {
            reason: question.excluded?.reason ?? "",
          })}
        </p>
      ) : null}
      <ContentView document={question.prompt} className="text-sm" />
      {isChoiceType(question.type) && question.options.length > 0 ? (
        <ul className="space-y-1">
          {question.options.map((option) => (
            <li key={option.id} className="flex items-start gap-2 text-sm">
              <span className="text-muted-foreground w-5 shrink-0 font-medium">
                {option.label}
              </span>
              <ContentView document={option.content} className="min-w-0 flex-1" />
              {keys.has(option.id) ? (
                <span className="text-success flex shrink-0 items-center gap-1 text-xs">
                  <Check className="size-3.5" aria-hidden="true" />
                  {t("imports.review.correct")}
                </span>
              ) : null}
            </li>
          ))}
        </ul>
      ) : null}
      <AnswerSummary question={question} />
    </article>
  );
});
