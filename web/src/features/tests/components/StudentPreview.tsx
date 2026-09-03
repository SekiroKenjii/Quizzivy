import { useTranslation } from "react-i18next";
import { Headphones } from "lucide-react";
import { Markdown } from "@/components/shared/Markdown";
import { Card } from "@/components/ui/card";
import { blankSlots } from "@/features/question-bank/blankSlots";
import type { components } from "@/lib/api/schema";

type StudentQuestion = components["schemas"]["StudentQuestion"];

/** A read-only rendering of what a student receives. */
export function StudentPreview({ questions }: { questions: StudentQuestion[] }) {
  const { t } = useTranslation();

  return (
    <ol className="space-y-3">
      {questions.map((question, index) => (
        <li key={question.id}>
          <Card className="gap-0 p-5">
            <div className="text-muted-foreground flex items-center gap-2 text-xs">
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

            <div className="mt-2 text-base leading-relaxed">
              <Markdown plugins={question.type === "fill_blank" ? [blankSlots] : []}>
                {question.prompt}
              </Markdown>
            </div>

            {question.options && question.options.length > 0 ? (
              <ul className="mt-3 space-y-2">
                {question.options.map((option, optionIndex) => (
                  <li
                    key={option.id}
                    className="flex items-center gap-2.5 rounded-md border px-3 py-2 text-sm"
                  >
                    <span className="text-muted-foreground w-4 shrink-0 text-center text-xs">
                      {String.fromCharCode(65 + optionIndex)}
                    </span>
                    {option.text}
                  </li>
                ))}
              </ul>
            ) : null}

            {question.type === "short_answer" ? (
              <p className="text-muted-foreground mt-3 rounded-md border border-dashed px-3 py-6 text-center text-xs">
                {t("preview.writtenAnswer")}
              </p>
            ) : null}
          </Card>
        </li>
      ))}
    </ol>
  );
}
