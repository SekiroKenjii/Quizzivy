import { QuestionProse } from "@/components/shared/content/QuestionProse";
import { OptionText } from "@/components/shared/content/OptionText";
import { useTranslation } from "react-i18next";
import { AudioPlayer } from "@/features/media/components/AudioPlayer";
import type { Answer } from "@/features/take-test/api";
import { PromptMedia } from "@/features/take-test/components/PromptMedia";
import { nfc } from "@/lib/nfc";
import { countWords } from "@/lib/words";
import { cn } from "@/lib/utils";
import type { AdminQuestion, ReviewAnswer } from "../api";
import { OPTION, optionKey } from "./answerStyles";
import { EssayMarksReview, MatchingReview, OverLimit } from "./AnswerReviewTypes";
import { tfngLabelKey } from "@/components/shared/content/trueFalse";
import type { TFunction } from "i18next";

type ReviewQuestion = Pick<
  AdminQuestion,
  | "type"
  | "prompt"
  | "promptContent"
  | "contentAssets"
  | "media"
  | "options"
  | "blanks"
  | "selectCount"
  | "maxWords"
  | "matchItems"
  | "rubric"
>;

type ShortAnswerPresentation = Readonly<{ label: string }>;

/**
 * AnswerReview preserves full-review defaults and optionally presents a short
 * answer with grading metrics and its supplied label. An answer the centre's
 * retention policy removed shows the notice that says so in place of the
 * response. The R6 types: true / false / not given by its position labels,
 * "Choose N" with its picks against the key, matching as pairs against the
 * key, an essay with its word count, band and criteria, and a short answer
 * past its word limit with the over-limit badge. The prompt's images draw
 * from `contentAssets`, signed for the grader (D-077).
 */
export function AnswerReview({
  question,
  answer,
  shortAnswerPresentation,
}: Readonly<{
  question: ReviewQuestion;
  answer: ReviewAnswer | undefined;
  shortAnswerPresentation?: ShortAnswerPresentation;
}>) {
  const { t } = useTranslation();
  const given = answer?.answer ?? null;
  return (
    <PromptMedia assets={question.contentAssets}>
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
        {answer?.answerRemoved === true ? (
          <p
            data-slot="answer-removed"
            className="text-muted-foreground text-sm italic"
          >
            {t("review.answerRemoved")}
          </p>
        ) : (
          <Body
            question={question}
            given={given}
            shortAnswerPresentation={shortAnswerPresentation}
          />
        )}
        {question.type === "essay" && answer?.answerRemoved !== true && (
          <EssayMarksReview question={question} answer={answer} />
        )}
      </div>
    </PromptMedia>
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
  const text =
    given !== null && given.type === "text" && given.value.trim() !== ""
      ? given.value
      : null;
  switch (question.type) {
    case "short_answer":
    case "essay":
      return (
        <div className={shortAnswerStyle.panel}>
          <div className="flex flex-wrap items-start justify-between gap-2">
            <p className={shortAnswerStyle.label}>
              {shortAnswerPresentation?.label ?? t("review.studentAnswer")}
            </p>
            {text !== null && question.type === "short_answer" && (
              <OverLimit text={text} maxWords={question.maxWords} />
            )}
          </div>
          {text === null ? (
            <p className="text-muted-foreground text-sm">{t("review.unanswered")}</p>
          ) : (
            <p className={shortAnswerStyle.answer}>{nfc(text)}</p>
          )}
          {text !== null && question.type === "essay" && (
            <p className="text-muted-foreground mt-2 text-xs tabular-nums">
              {t("review.wordCount", { count: countWords(text) })}
            </p>
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
    case "matching":
      return <MatchingReview question={question} given={given} />;
    default: {
      const chosen = chosenOptions(question, given);
      return (
        <div className="space-y-2">
          {question.type === "multiple_choice" && question.selectCount != null && (
            <p className="text-muted-foreground text-xs">
              {chosen.size > question.selectCount
                ? t("review.chooseOver", {
                    n: question.selectCount,
                    picked: chosen.size,
                  })
                : t("review.chooseCount", {
                    n: question.selectCount,
                    correct: (question.options ?? []).filter(
                      (option) => option.isCorrect && chosen.has(option.id),
                    ).length,
                  })}
            </p>
          )}
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
                  {question.type === "tfng" && tfngLabelKey(index) !== null ? (
                    t(tfngLabelKey(index)!)
                  ) : (
                    <OptionText
                      text={option.text}
                      content={option.content}
                      type={question.type}
                    />
                  )}
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
