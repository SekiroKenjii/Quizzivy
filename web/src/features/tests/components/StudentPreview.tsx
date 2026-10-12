import { AudioPlayer } from "@/features/media/components/AudioPlayer";
import { useId, useMemo } from "react";
import { PreviewGroupContext } from "./PreviewGroupContext";
import { useTranslation } from "react-i18next";
import { Markdown } from "@/components/shared/Markdown";
import { DeckScale } from "@/components/ui/deck-scale";
import { QuestionSheet } from "@/features/take-test/components/QuestionSheet";
import type { components } from "@/lib/api/schema";
import { cn } from "@/lib/utils";

type StudentQuestion = components["schemas"]["StudentQuestion"];

/**
 * PreviewMark flags a question of the preview as new or changed against
 * another version: `tone` picks the outline and the chip's colours, `label`
 * is the chip's text.
 */
export type PreviewMark = Readonly<{ tone: "added" | "changed"; label: string }>;

const MARK_OUTLINE: Record<PreviewMark["tone"], string> = {
  added: "outline-success-ink",
  changed: "outline-warning-ink",
};

const MARK_CHIP: Record<PreviewMark["tone"], string> = {
  added: "bg-success-soft text-success-ink",
  changed: "bg-warning-soft text-warning-ink",
};

function ignoreAnswer() {
  return undefined;
}

/**
 * StudentPreview renders frozen questions and their shared context as the
 * student's paper draws them: the engine's passage and question panes, one
 * under the other, on a deck surface. Nothing can be answered: every control
 * is disabled and no answer is written. `marks`, keyed by question id,
 * outlines a question with a dashed line and names it in a chip.
 */
export function StudentPreview({
  questions,
  sections = [],
  groups = [],
  marks,
  onRetryMedia,
}: Readonly<{
  questions: StudentQuestion[];
  sections?: components["schemas"]["StudentSection"][];
  groups?: components["schemas"]["StudentGroup"][];
  marks?: ReadonlyMap<string, PreviewMark> | undefined;
  onRetryMedia?: (() => void) | undefined;
}>) {
  const { t } = useTranslation();
  const prefix = useId();
  const questionAnchor = (id: string) => `${prefix}-question-${id}`;
  const numbers = useMemo(
    () => new Map(questions.map((question, index) => [question.id, index + 1])),
    [questions],
  );
  const contexts = new Map(groups.map((group) => [group.questionIds[0], group]));
  const sectionById = new Map(sections.map((section) => [section.id, section]));

  return (
    <DeckScale>
      <ol className="flex min-w-0 flex-col gap-4">
        {questions.map((question, index) => {
          const group = contexts.get(question.id);
          const section = sectionById.get(question.sectionId);
          const mark = marks?.get(question.id);
          return (
            <li key={question.id} className="flex min-w-0 flex-col gap-3">
              {section && questions[index - 1]?.sectionId !== section.id ? (
                <header className="flex flex-col gap-2 pt-3">
                  <h2 className="font-semibold">{section.title}</h2>
                  {section.instructions ? (
                    <Markdown>{section.instructions}</Markdown>
                  ) : null}
                </header>
              ) : null}
              {group ? (
                <PreviewGroupContext
                  group={group}
                  numbers={numbers}
                  questionAnchor={questionAnchor}
                  onRetryMedia={onRetryMedia}
                />
              ) : null}
              <div
                data-preview-mark={mark?.tone}
                className={cn(
                  "bg-sidebar min-w-0 rounded-xl border px-4 py-4.5",
                  mark &&
                    `outline-[1.5px] outline-offset-[6px] outline-dashed ${MARK_OUTLINE[mark.tone]}`,
                )}
              >
                <QuestionSheet
                  question={question}
                  number={index + 1}
                  total={questions.length}
                  headingId={questionAnchor(question.id)}
                  action={
                    mark ? (
                      <span
                        className={cn(
                          "text-2xs inline-flex h-5 flex-none items-center rounded-[6px] px-1.75 font-semibold whitespace-nowrap",
                          MARK_CHIP[mark.tone],
                        )}
                      >
                        {mark.label}
                      </span>
                    ) : undefined
                  }
                  audio={
                    question.media?.kind === "audio" && question.media.url ? (
                      <AudioPlayer
                        src={question.media.url}
                        label={t("preview.questionNumber", { n: index + 1 })}
                        durationMs={question.media.durationMs}
                        allowSeek={question.audio?.allowSeek ?? false}
                        hint={t("preview.sharedAudioHint")}
                        onRetry={onRetryMedia}
                      />
                    ) : null
                  }
                  answer={undefined}
                  onAnswer={ignoreAnswer}
                  disabled
                  onRetryMedia={onRetryMedia}
                />
              </div>
            </li>
          );
        })}
      </ol>
    </DeckScale>
  );
}
