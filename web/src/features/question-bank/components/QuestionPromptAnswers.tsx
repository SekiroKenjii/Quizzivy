import { useState } from "react";
import type { Editor } from "@tiptap/react";
import { useTranslation } from "react-i18next";
import { insertGap } from "@/components/shared/content/editor/gapCommands";
import { nextBlankOrdinal } from "@/features/question-bank/blankContent";
import type { QuestionValues } from "@/features/question-bank/questionSchema";
import { AnswerArea } from "./AnswerArea";
import { BlankPromptField } from "./BlankPromptField";
import { QuestionProseField } from "./QuestionProseField";

/**
 * QuestionPromptAnswers is the prompt and the answer area of one question,
 * as every editor of a question draws them: the prompt in the content editor
 * (a fill-in-the-blank's with its gaps) under "Write the question students
 * will read", then the type's answers and grading note, whose "Insert gap"
 * places a gap at the prompt's caret. `clearPromptOnFocus` opens a prompt
 * that still holds the starter text empty.
 */
export function QuestionPromptAnswers({
  value,
  clearPromptOnFocus = false,
  onChange,
}: Readonly<{
  value: QuestionValues;
  clearPromptOnFocus?: boolean;
  onChange: (value: QuestionValues) => void;
}>) {
  const { t } = useTranslation();
  const [promptEditor, setPromptEditor] = useState<Editor | null>(null);
  const placeholder = t("questionEditor.promptPlaceholder");
  return (
    <>
      {value.type === "fill_blank" ? (
        <BlankPromptField
          value={value}
          onChange={onChange}
          onEditor={setPromptEditor}
          placeholder={placeholder}
        />
      ) : (
        <QuestionProseField
          id="question-prompt"
          text={value.prompt}
          content={value.promptContent}
          label={t("questionEditor.prompt")}
          prompt
          clearOnFocus={clearPromptOnFocus}
          placeholder={placeholder}
          onChange={(prompt, promptContent) =>
            onChange({ ...value, prompt, promptContent })
          }
        />
      )}
      <AnswerArea
        value={value}
        onChange={onChange}
        onInsertGap={
          promptEditor
            ? () =>
                insertGap(promptEditor, () => String(nextBlankOrdinal(value.blanks)))
            : undefined
        }
      />
    </>
  );
}

/**
 * ExplanationField is a question's explanation in the content editor, with
 * the rule for when students see it as its hint.
 */
export function ExplanationField({
  value,
  onChange,
}: Readonly<{
  value: QuestionValues;
  onChange: (value: QuestionValues) => void;
}>) {
  const { t } = useTranslation();
  return (
    <QuestionProseField
      id="question-explanation"
      text={value.explanation ?? ""}
      content={value.explanationContent}
      label={t("questionEditor.explanation")}
      hint={t("questionEditor.explanationHint")}
      placeholder={t("questionEditor.explanationPlaceholder")}
      onChange={(explanation, explanationContent) =>
        onChange({ ...value, explanation, explanationContent })
      }
    />
  );
}
