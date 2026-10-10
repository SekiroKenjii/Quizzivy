import { Fragment } from "react";
import type { TFunction } from "i18next";
import { useTranslation } from "react-i18next";
import {
  Check,
  Hourglass,
  Minus,
  Percent,
  ScrollText,
  X,
  type LucideIcon,
} from "lucide-react";
import { Markdown } from "@/components/shared/Markdown";
import { OptionText } from "@/components/shared/content/OptionText";
import { QuestionProse } from "@/components/shared/content/QuestionProse";
import { RichBlankPrompt } from "@/components/shared/content/RichBlankPrompt";
import { AudioPlayer } from "@/features/media/components/AudioPlayer";
import { blankInputs } from "@/features/take-test/components/blankInputs";
import { QuestionImage } from "@/features/take-test/components/QuestionImage";
import type { Locale } from "@/lib/i18n";
import { cn } from "@/lib/utils";
import type { AttemptResult, ResultQuestion } from "../api";
import {
  correctKey,
  given,
  verdict,
  type Given,
  type Key,
  type Verdict,
} from "../resultView";

const MARK: Record<
  Verdict,
  Readonly<{ icon: LucideIcon; tone: string; label: string | null }>
> = {
  correct: {
    icon: Check,
    tone: "bg-success-soft text-success-ink",
    label: "result.verdict.correct",
  },
  partial: {
    icon: Percent,
    tone: "bg-brand-soft text-brand-ink",
    label: "result.verdict.partial",
  },
  wrong: {
    icon: X,
    tone: "bg-danger-soft text-danger-ink",
    label: "result.verdict.wrong",
  },
  waiting: { icon: Hourglass, tone: "bg-warning-soft text-warning-ink", label: null },
  unknown: { icon: Minus, tone: "bg-muted text-muted-fg", label: null },
};

const PROSE = "min-w-0 flex-1 text-base leading-normal! font-medium text-pretty";
const LINE = "text-ui leading-normal";
const GAP = "___";
const BETWEEN_OPTIONS = ", ";
const BETWEEN_BLANKS = " · ";
const EMPTY_BLANK = "—";

function Blank() {
  const { t } = useTranslation();
  return (
    <span role="img" aria-label={t("result.blank")}>
      {GAP}
    </span>
  );
}

function Prompt({ question }: Readonly<{ question: ResultQuestion }>) {
  if (question.type !== "fill_blank")
    return (
      <QuestionProse
        className={PROSE}
        text={question.prompt}
        content={question.promptContent}
      />
    );
  if (question.promptContent != null)
    return (
      <RichBlankPrompt
        className={PROSE}
        text={question.prompt}
        content={question.promptContent}
        blanks={question.blanks ?? []}
        renderBlank={() => <Blank />}
      />
    );
  return (
    <Markdown
      className={PROSE}
      plugins={[blankInputs]}
      components={{
        span: ({ node, ...rest }) =>
          node?.properties?.["data-blank"] == null ? <span {...rest} /> : <Blank />,
      }}
    >
      {question.prompt}
    </Markdown>
  );
}

function Options({
  options,
  type,
}: Readonly<{ options: Key & { kind: "options" }; type: string }>) {
  return options.options.map((option, index) => (
    <Fragment key={option.id}>
      {index > 0 && BETWEEN_OPTIONS}
      <OptionText text={option.text} content={option.content} type={type} />
    </Fragment>
  ));
}

function GivenText({ answer, type }: Readonly<{ answer: Given; type: string }>) {
  const { t } = useTranslation();
  switch (answer.kind) {
    case "none":
      return <>{t("result.noAnswer")}</>;
    case "options":
      return <Options options={answer} type={type} />;
    case "text":
      return <span className="whitespace-pre-wrap">{answer.text}</span>;
    case "boolean":
      return <>{t(answer.value ? "result.true" : "result.false")}</>;
    case "blanks":
      return (
        <>{answer.values.map((value) => value ?? EMPTY_BLANK).join(BETWEEN_BLANKS)}</>
      );
  }
}

function points(
  mark: Verdict,
  question: ResultQuestion,
  locale: Locale,
  t: TFunction,
): string | null {
  if (mark === "waiting") return t("result.waiting");
  if (mark === "unknown") return null;
  const n = new Intl.NumberFormat(locale, { maximumFractionDigits: 2 });
  return t("result.points", {
    earned: n.format(question.earned ?? 0),
    total: n.format(question.points),
  });
}

/**
 * ReviewItem is one answer under "Your answers", as the design deck draws it:
 * a status circle, the number and the prompt with what the answer earned,
 * the question's image, then "You answered" and, where the review policy allows, the correct answer
 * and the explanation. The circle and the points come from the mark alone, so
 * a hidden score shows a dash and no points, and the answer is struck through
 * in danger only when it was marked and earned nothing. A question's own
 * recording, its transcript by the recording's own rule, and the grader's
 * comment follow. Nothing here is read from a field the policy removed.
 */
export function ReviewItem({
  question,
  number,
  review,
  onRetry,
}: Readonly<{
  question: ResultQuestion;
  number: number;
  review: AttemptResult["review"];
  onRetry: () => void;
}>) {
  const { t, i18n } = useTranslation();
  const mark = verdict(question, review);
  const { icon: Icon, tone, label } = MARK[mark];
  const answer = given(question);
  const key = correctKey(question, review);
  const earned = points(mark, question, i18n.language as Locale, t);
  const numbered = `${number}. `;
  const struck = mark === "wrong" && answer.kind !== "none";
  const audio = question.media?.kind === "audio" ? question.media : null;
  const explanation = review.showExplanations ? (question.explanation ?? null) : null;

  return (
    <article
      data-verdict={mark}
      className="bg-card flex gap-3 rounded-xl border px-4 py-3.5"
    >
      <span
        className={cn("grid size-6.5 flex-none place-items-center rounded-full", tone)}
      >
        <Icon aria-hidden="true" className="size-3.5" />
        {label !== null && <span className="sr-only">{t(label)}</span>}
      </span>
      <div className="flex min-w-0 flex-1 flex-col gap-1.5 break-words">
        <div className="flex justify-between gap-2.5">
          <div className="min-w-0 flex-1">
            <span className="float-left text-base leading-normal font-medium whitespace-pre">
              {numbered}
            </span>
            <Prompt question={question} />
          </div>
          {earned !== null && (
            <span className="text-muted-fg text-meta leading-normal whitespace-nowrap">
              {earned}
            </span>
          )}
        </div>
        <QuestionImage question={question} onRetry={onRetry} />
        <p className={LINE}>
          <span className="text-muted-fg">{t("result.youAnswered")} </span>
          <span
            data-slot="given"
            className={cn("font-medium", struck && "text-danger-ink line-through")}
          >
            <GivenText answer={answer} type={question.type} />
          </span>
        </p>
        {key !== null && (
          <p className={LINE}>
            <span className="text-muted-fg">{t("result.correctAnswer")} </span>
            <span className="text-success-ink font-medium">
              {key.kind === "options" ? (
                <Options options={key} type={question.type} />
              ) : (
                key.values.join(BETWEEN_BLANKS)
              )}
            </span>
          </p>
        )}
        {explanation !== null && (
          <div className="bg-muted text-muted-fg rounded-md px-2.5 py-2 text-sm">
            <QuestionProse
              className="leading-normal!"
              text={explanation}
              content={question.explanationContent}
            />
          </div>
        )}
        {audio !== null && (
          <>
            <AudioPlayer
              src={audio.url}
              label={t("takeTest.audioLabel")}
              durationMs={audio.durationMs}
              allowSeek
              size="sm"
              preload="metadata"
              hint={t("result.replayFreely")}
              onRetry={onRetry}
            />
            {question.transcript != null ? (
              <details className="text-sm">
                <summary className="text-muted-fg flex cursor-pointer items-center gap-1.5">
                  <ScrollText aria-hidden="true" className="size-4" />
                  {t("result.showTranscript")}
                </summary>
                <p className="text-muted-fg mt-2 leading-relaxed whitespace-pre-wrap">
                  {question.transcript}
                </p>
              </details>
            ) : (
              <p className="text-muted-fg flex items-center gap-1.5 text-xs">
                <ScrollText aria-hidden="true" className="size-3.5 flex-none" />
                <span>{t("result.noTranscript")}</span>
              </p>
            )}
          </>
        )}
        {question.graderComment != null && question.graderComment.trim() !== "" && (
          <div className="bg-brand-soft rounded-md px-2.5 py-2 text-sm">
            <span className="block font-semibold">{t("result.teacherComment")}</span>
            <span className="block whitespace-pre-wrap">{question.graderComment}</span>
          </div>
        )}
      </div>
    </article>
  );
}
