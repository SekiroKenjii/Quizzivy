import { useTranslation } from "react-i18next";
import type { Answer } from "@/features/take-test/api";
import { formatBand } from "@/lib/bands";
import { useLocale } from "@/lib/i18n/useLocale";
import { nfc } from "@/lib/nfc";
import { cn } from "@/lib/utils";
import { countWords } from "@/lib/words";
import type { AdminQuestion, ReviewAnswer } from "../api";
import { OPTION } from "./answerStyles";

/**
 * OverLimit is the badge a short answer past its word limit carries, counted
 * by the engine's own counter: "Over the word limit (5 of 3)". Nothing when
 * the question has no limit or the answer keeps to it.
 */
export function OverLimit({
  text,
  maxWords,
}: Readonly<{ text: string; maxWords: number | null | undefined }>) {
  const { t } = useTranslation();
  if (maxWords == null) return null;
  const words = countWords(text);
  if (words <= maxWords) return null;
  return (
    <span className="bg-warning-soft text-warning-ink inline-flex h-5.5 items-center rounded-full px-2 text-xs font-medium whitespace-nowrap">
      {t("review.overLimit", { words, max: maxWords })}
    </span>
  );
}

/**
 * MatchingReview lists a matching answer's pairs against its key: each item
 * with the choice the student gave it, tinted right or wrong, and the right
 * choice beside a wrong or missing one.
 */
export function MatchingReview({
  question,
  given,
}: Readonly<{
  question: Pick<AdminQuestion, "options" | "matchItems">;
  given: Answer | null;
}>) {
  const { t } = useTranslation();
  const pairs = given?.type === "matching" ? given.pairs : {};
  const options = new Map(
    (question.options ?? []).map((option) => [option.id, option]),
  );
  const label = (id: string | undefined) => {
    const option = id === undefined ? undefined : options.get(id);
    if (!option) return null;
    return option.key ? `${option.key} · ${nfc(option.text)}` : nfc(option.text);
  };
  return (
    <ul className="space-y-2">
      {(question.matchItems ?? []).map((item) => {
        const picked = pairs[item.id];
        const right = picked === item.correctOptionId;
        const pickedKey = right ? "review.matchedRight" : "review.matchedTo";
        return (
          <li
            key={item.id}
            className={cn(
              OPTION.base,
              picked !== undefined && (right ? OPTION.correct : OPTION.wrong),
            )}
          >
            <span className={OPTION.key}>{item.key}</span>
            <div className="min-w-0 flex-1 text-sm">
              <p>{nfc(item.text)}</p>
              <p className="mt-1">
                {picked === undefined ? (
                  <span className="text-muted-foreground">
                    {t("review.notMatched")}
                  </span>
                ) : (
                  t(pickedKey, { choice: label(picked) })
                )}
              </p>
              {!right && (
                <p className="text-muted-foreground mt-1 text-xs">
                  {t("review.matchKey", { choice: label(item.correctOptionId) })}
                </p>
              )}
            </div>
          </li>
        );
      })}
    </ul>
  );
}

/**
 * EssayMarksReview shows a graded essay's band and, with a rubric, the band
 * each criterion was given, in authored order.
 */
export function EssayMarksReview({
  question,
  answer,
}: Readonly<{
  question: Pick<AdminQuestion, "rubric">;
  answer: Pick<ReviewAnswer, "band" | "criteria"> | undefined;
}>) {
  const { t } = useTranslation();
  const locale = useLocale();
  if (answer?.band == null) return null;
  const given = new Map(
    (answer.criteria ?? []).map((entry) => [entry.criterionId, entry.band]),
  );
  return (
    <div className="space-y-1.5 text-sm">
      <p className="font-semibold tabular-nums">
        {t("review.essayBand", { band: formatBand(answer.band, locale) })}
      </p>
      {(question.rubric ?? []).length > 0 && (
        <dl className="grid max-w-sm grid-cols-[1fr_auto] gap-x-4 gap-y-1">
          {(question.rubric ?? []).map((criterion) => {
            const band = given.get(criterion.id);
            return (
              <div key={criterion.id} className="contents">
                <dt className="text-muted-foreground">{criterion.label}</dt>
                <dd className="tabular-nums">
                  {band === undefined ? "—" : formatBand(band, locale)}
                </dd>
              </div>
            );
          })}
        </dl>
      )}
    </div>
  );
}
