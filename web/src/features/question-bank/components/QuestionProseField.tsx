import { lazy, Suspense, useState } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { Textarea } from "@/components/ui/textarea";
import { QuestionProse } from "@/components/shared/content/QuestionProse";
import type {
  QuestionPromptContent,
  QuestionContent,
} from "@/components/shared/content/questionContent";
import { PromptField } from "./PromptField";

const RichProseEditor = lazy(() =>
  import("./RichProseEditor").then((module) => ({ default: module.RichProseEditor })),
);

/** QuestionProseField keeps historical Markdown editable and loads rich authoring only on request. */
export function QuestionProseField({
  text,
  content,
  id,
  label,
  prompt = false,
  clearOnFocus = false,
  onChange,
}: Readonly<{
  text: string;
  content?: QuestionPromptContent | null | undefined;
  id: string;
  label: string;
  prompt?: boolean;
  clearOnFocus?: boolean;
  onChange: (text: string, content: QuestionContent | null) => void;
}>) {
  const { t } = useTranslation();
  const [editing, setEditing] = useState(false);
  if (editing)
    return (
      <Suspense
        fallback={
          <Skeleton className="h-64 w-full" aria-label={t("contentEditor.loading")} />
        }
      >
        <RichProseEditor
          text={
            prompt && clearOnFocus && text === t("builder.starterPrompt") ? "" : text
          }
          content={content ?? null}
          id={id}
          label={label}
          onChange={onChange}
          onClose={() => setEditing(false)}
        />
      </Suspense>
    );
  const legacyField = prompt ? (
    <PromptField
      id={id}
      value={text}
      clearOnFocus={clearOnFocus}
      onChange={(value) => onChange(value, null)}
    />
  ) : (
    <Textarea
      id={id}
      value={text}
      className="min-h-14"
      onChange={(event) => onChange(event.target.value, null)}
    />
  );
  return (
    <div className="flex flex-col gap-2">
      {content == null ? (
        legacyField
      ) : (
        <QuestionProse
          text={text}
          content={content}
          className="rounded-md border p-4"
        />
      )}
      {(content != null || import.meta.env.VITE_RICH_QUESTION_EDITOR === "true") && (
        <Button
          type="button"
          variant="outline"
          size="sm"
          className="self-start"
          onClick={() => setEditing(true)}
        >
          {t(
            content == null ? "questionEditor.formatProse" : "questionEditor.editProse",
            { field: label },
          )}
        </Button>
      )}
    </div>
  );
}
