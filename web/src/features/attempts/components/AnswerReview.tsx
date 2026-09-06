import { useTranslation } from "react-i18next";
import { Markdown } from "@/components/shared/Markdown";
import { AudioPlayer } from "@/features/media/components/AudioPlayer";
import type { Answer } from "@/features/take-test/api";
import { cn } from "@/lib/utils";
import type { AdminQuestion, ReviewAnswer } from "../api";
import { OPTION, optionKey } from "./answerStyles";
import type { TFunction } from "i18next";

/**
 * A question as the teacher reads it after the fact: the prompt, the
 * student's answer against the key, and the audio if there was any (G-03).
 */
export function AnswerReview({
  question,
  answer,
}: Readonly<{
  question: AdminQuestion;
  answer: ReviewAnswer | undefined;
}>) {
  const given = answer?.answer ?? null;
  return (
    <div className="space-y-4">
      <Markdown className="text-sm">{question.prompt}</Markdown>
      {question.media?.kind === "audio" && (
        <AudioPlayer
          src={question.media.url}
          label={question.media.originalFilename}
          durationMs={question.media.durationMs}
          allowSeek
          size="sm"
          preload="metadata"
        />
      )}
      <Body question={question} given={given} />
    </div>
  );
}

function Body({
  question,
  given,
}: Readonly<{ question: AdminQuestion; given: Answer | null }>) {
  const { t } = useTranslation();
  switch (question.type) {
    case "short_answer":
      return (
        <div className="rounded-md border p-4">
          <p className="text-muted-foreground mb-2 text-xs">
            {t("review.studentAnswer")}
          </p>
          {given !== null && "value" in given && String(given.value).trim() !== "" ? (
            <p className="text-base leading-relaxed whitespace-pre-wrap">
              {String(given.value)}
            </p>
          ) : (
            <p className="text-muted-foreground text-sm">{t("review.unanswered")}</p>
          )}
        </div>
      );
    case "fill_blank": {
      const values = given !== null && "values" in given ? given.values : {};
      return (
        <div className="space-y-2">
          {(question.blanks ?? []).map((blank) => {
            const typed = values[blank.id] ?? "";
            const hit = matches(typed, blank.acceptedAnswers, blank.caseSensitive);
            return (
              <div key={blank.id} className={cn(OPTION.base, blankTone(typed, hit))}>
                <span className={OPTION.key}>{blank.ordinal}</span>
                <div className="min-w-0 flex-1 text-sm">
                  {typed === "" ? (
                    <span className="text-muted-foreground">
                      {t("review.unanswered")}
                    </span>
                  ) : (
                    <span>{typed}</span>
                  )}
                  <p className="text-muted-foreground mt-1 text-xs">
                    {t("review.accepted", {
                      answers: blank.acceptedAnswers.join(" · "),
                    })}
                  </p>
                </div>
              </div>
            );
          })}
        </div>
      );
    }
    default: {
      const chosen = new Set(
        given !== null && "optionIds" in given ? given.optionIds : [],
      );
      return (
        <div className="space-y-2">
          {(question.options ?? []).map((option, index) => {
            const picked = chosen.has(option.id);
            return (
              <div
                key={option.id}
                className={cn(
                  OPTION.base,
                  option.isCorrect && OPTION.correct,
                  picked && !option.isCorrect && OPTION.wrong,
                )}
              >
                <span className={OPTION.key}>{optionKey(index)}</span>
                <span className="text-sm">{option.text}</span>
                <span className="text-muted-foreground ml-auto self-center text-xs">
                  {optionNote(picked, option.isCorrect, t)}
                </span>
              </div>
            );
          })}
        </div>
      );
    }
  }
}

/** grading.normalise's rule: whitespace and case forgiven per the blank, the answer not. */
function matches(typed: string, accepted: string[], caseSensitive: boolean): boolean {
  const fold = (s: string) => {
    const collapsed = s.normalize("NFC").trim().split(/\s+/).join(" ");
    return caseSensitive ? collapsed : collapsed.toLocaleLowerCase();
  };
  const given = fold(typed);
  return given !== "" && accepted.some((a) => fold(a) === given);
}

function blankTone(typed: string, hit: boolean): string {
  if (typed === "") return "";
  return hit ? OPTION.correct : OPTION.wrong;
}

function optionNote(picked: boolean, isCorrect: boolean, t: TFunction): string | null {
  if (picked) return t(isCorrect ? "review.pickedCorrect" : "review.picked");
  return isCorrect ? t("review.correctAnswer") : null;
}
