import { PreviewImage } from "./PreviewImage";
import { AudioPlayer } from "@/features/media/components/AudioPlayer";
import { useId, useMemo } from "react";
import { PreviewGroupContext } from "./PreviewGroupContext";
import { QuestionProse } from "@/components/shared/content/QuestionProse";
import { OptionText } from "@/components/shared/content/OptionText";
import { useTranslation } from "react-i18next";
import { Headphones } from "lucide-react";
import { Markdown } from "@/components/shared/Markdown";
import { Card, CardHeader, CardContent } from "@/components/ui/card";
import { blankSlots } from "@/features/question-bank/blankSlots";
import type { components } from "@/lib/api/schema";

type StudentQuestion = components["schemas"]["StudentQuestion"];

/** StudentPreview renders frozen questions and their shared context without answer controls. */
export function StudentPreview({
  questions,
  sections = [],
  groups = [],
  onRetryMedia,
}: Readonly<{
  questions: StudentQuestion[];
  sections?: components["schemas"]["StudentSection"][];
  groups?: components["schemas"]["StudentGroup"][];
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
    <ol className="flex min-w-0 flex-col gap-4">
      {questions.map((question, index) => {
        const group = contexts.get(question.id);
        const section = sectionById.get(question.sectionId);
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
            <Card className="min-w-0 gap-3 py-5">
              <CardHeader className="px-5">
                <div
                  id={questionAnchor(question.id)}
                  tabIndex={-1}
                  className="text-muted-foreground flex scroll-mt-6 items-center gap-2 rounded-sm text-xs"
                >
                  <span className="tabular-nums">
                    {t("preview.questionNumber", { n: index + 1 })}
                  </span>
                  {question.media?.kind === "audio" ? (
                    <Headphones className="size-3.5" aria-hidden="true" />
                  ) : null}
                  <span className="ml-auto tabular-nums">
                    {t("builder.points", { points: question.points })}
                  </span>
                </div>
              </CardHeader>
              <CardContent className="min-w-0 px-5">
                <div className="text-base leading-relaxed">
                  {question.type === "fill_blank" && question.promptContent == null ? (
                    <Markdown plugins={[blankSlots]}>{question.prompt}</Markdown>
                  ) : (
                    <QuestionProse
                      text={question.prompt}
                      content={question.promptContent}
                    />
                  )}
                </div>

                {question.media?.kind === "image" && question.media.url ? (
                  <div className="mt-3">
                    <PreviewImage
                      src={question.media.url}
                      alt={question.media.originalFilename}
                      onRetry={onRetryMedia}
                    />
                  </div>
                ) : null}
                {question.media?.kind === "audio" && question.media.url ? (
                  <div className="mt-3">
                    <AudioPlayer
                      src={question.media.url}
                      label={t("preview.questionNumber", { n: index + 1 })}
                      durationMs={question.media.durationMs}
                      allowSeek={question.audio?.allowSeek ?? false}
                      hint={t("preview.sharedAudioHint")}
                      onRetry={onRetryMedia}
                    />
                  </div>
                ) : null}

                {question.options && question.options.length > 0 ? (
                  <ul className="mt-3 flex flex-col gap-2">
                    {question.options.map((option, optionIndex) => (
                      <li
                        key={option.id}
                        className="flex items-center gap-2.5 rounded-md border px-3 py-2 text-sm"
                      >
                        <span className="text-muted-foreground w-4 shrink-0 text-center text-xs">
                          {String.fromCharCode(65 + optionIndex)}
                        </span>
                        <OptionText text={option.text} content={option.content} />
                      </li>
                    ))}
                  </ul>
                ) : null}

                {question.type === "short_answer" ? (
                  <p className="text-muted-foreground mt-3 rounded-md border border-dashed px-3 py-6 text-center text-xs">
                    {t("preview.writtenAnswer")}
                  </p>
                ) : null}
              </CardContent>
            </Card>
          </li>
        );
      })}
    </ol>
  );
}
