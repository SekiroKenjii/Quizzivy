import { lazy, Suspense, useState } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { QuestionProse } from "@/components/shared/content/QuestionProse";
import type { QuestionValues } from "../questionSchema";
import { PromptField } from "./PromptField";

const RichBlankEditor = lazy(() =>
  import("./RichBlankEditor").then((module) => ({ default: module.RichBlankEditor })),
);

/** BlankPromptField loads rich blank authoring on demand while retaining legacy Markdown editing. */
export function BlankPromptField({
  value,
  onChange,
}: Readonly<{ value: QuestionValues; onChange: (value: QuestionValues) => void }>) {
  const { t } = useTranslation();
  const [editing, setEditing] = useState(false);
  if (editing)
    return (
      <Suspense
        fallback={
          <Skeleton className="h-64 w-full" aria-label={t("contentEditor.loading")} />
        }
      >
        <RichBlankEditor
          value={value}
          onChange={onChange}
          onClose={() => setEditing(false)}
        />
      </Suspense>
    );
  return (
    <div className="flex flex-col gap-2">
      {value.promptContent == null ? (
        <PromptField
          id="question-prompt"
          value={value.prompt}
          onChange={(prompt) => onChange({ ...value, prompt })}
        />
      ) : (
        <QuestionProse
          text={value.prompt}
          content={value.promptContent}
          className="rounded-md border p-4"
        />
      )}
      {(value.promptContent != null ||
        import.meta.env.VITE_RICH_QUESTION_EDITOR === "true") && (
        <Button
          type="button"
          variant="outline"
          size="sm"
          className="self-start"
          onClick={() => setEditing(true)}
        >
          {t(
            value.promptContent == null
              ? "questionEditor.formatProse"
              : "questionEditor.editProse",
            { field: t("questionEditor.prompt") },
          )}
        </Button>
      )}
    </div>
  );
}
