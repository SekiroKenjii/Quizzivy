import { QuestionProse } from "@/components/shared/content/QuestionProse";
import { OptionText } from "@/components/shared/content/OptionText";
import { useTranslation } from "react-i18next";
import { AudioPlayer } from "@/features/media/components/AudioPlayer";
import type { Answer } from "@/features/take-test/api";
import { nfc } from "@/lib/nfc";
import { cn } from "@/lib/utils";
import type { AdminQuestion, ReviewAnswer } from "../api";
import { OPTION, optionKey } from "./answerStyles";
import type { TFunction } from "i18next";

type ReviewQuestion = Pick<
  AdminQuestion,
  "type" | "prompt" | "promptContent" | "media" | "options" | "blanks"
>;

type ShortAnswerPresentation = Readonly<{ label: string }>;

/** AnswerReview preserves full-review defaults and optionally presents a short answer with grading metrics and its supplied label. */
export function AnswerReview({
  question,
  answer,
  shortAnswerPresentation,
}: Readonly<{
  question: ReviewQuestion;
  answer: ReviewAnswer | undefined;
  shortAnswerPresentation?: ShortAnswerPresentation;
}>) {
  const given = answer?.answer ?? null;
  return (
    <div className="space-y-4">
      <QuestionProse
        className="text-sm"
        text={question.prompt}
        content={question.promptContent}
      />
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
      <Body
        question={question}
        given={given}
        shortAnswerPresentation={shortAnswerPresentation}
      />
    </div>
  );
}

function Body({
  question,
  given,
  shortAnswerPresentation,
}: Readonly<{
  question: ReviewQuestion;
  given: Answer | null;
  shortAnswerPresentation: ShortAnswerPresentation | undefined;
}>) {
  const { t } = useTranslation();
  const shortAnswerStyle = shortAnswerPresentation
    ? {
        panel: "bg-muted rounded-[10px] px-4 py-3.5",
        label: "text-muted-foreground mb-1.5 text-xs leading-[1.5] font-medium",
        answer: "text-[16px] leading-[1.65] [text-wrap:pretty] whitespace-pre-wrap",
      }
    : {
        panel: "rounded-md border p-4",
        label: "text-muted-foreground mb-2 text-xs",
        answer: "text-base leading-relaxed whitespace-pre-wrap",
      };
  switch (question.type) {
    case "short_answer":
      return (
        <div className={shortAnswerStyle.panel}>
          <p className={shortAnswerStyle.label}>
            {shortAnswerPresentation?.label ?? t("review.studentAnswer")}
          </p>
          {given !== null && "value" in given && String(given.value).trim() !== "" ? (
            <p className={shortAnswerStyle.answer}>{nfc(String(given.value))}</p>
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
                    <span>{nfc(typed)}</span>
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
      const chosen = chosenOptions(question, given);
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
                <span className="text-sm">
                  <OptionText
                    text={option.text}
                    content={option.content}
                    type={question.type}
                  />
                </span>
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

const TRUE_ORDINAL = 0;
const FALSE_ORDINAL = 1;

function chosenOptions(question: ReviewQuestion, given: Answer | null): Set<string> {
  if (given === null) return new Set();
  if ("optionIds" in given) return new Set(given.optionIds);
  if (question.type !== "true_false" || given.type !== "true_false") return new Set();
  const ordinal = given.value ? TRUE_ORDINAL : FALSE_ORDINAL;
  const option = question.options?.find((item) => item.ordinal === ordinal);
  return new Set(option === undefined ? [] : [option.id]);
}

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
