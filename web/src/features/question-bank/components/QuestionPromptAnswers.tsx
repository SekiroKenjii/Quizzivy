import { useState } from "react";
import type { Editor } from "@tiptap/react";
import { useTranslation } from "react-i18next";
import { insertGap } from "@/components/shared/content/editor/gapCommands";
import { nextBlankOrdinal } from "@/features/question-bank/blankContent";
import type { QuestionAi } from "@/features/question-bank/questionAi";
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
 * that still holds the starter text empty. `promptLabel` names the prompt,
 * "Prompt" unless the page names it otherwise. `ai` offers "Suggest wrong
 * options" on a choice question's options.
 */
export function QuestionPromptAnswers({
  value,
  clearPromptOnFocus = false,
  promptLabel,
  ai,
  onChange,
}: Readonly<{
  value: QuestionValues;
  clearPromptOnFocus?: boolean;
  promptLabel?: string | undefined;
  ai?: QuestionAi | undefined;
  onChange: (value: QuestionValues) => void;
}>) {
  const { t } = useTranslation();
  const [promptEditor, setPromptEditor] = useState<Editor | null>(null);
  const placeholder = t("questionEditor.promptPlaceholder");
  const label = promptLabel ?? t("questionEditor.prompt");
  return (
    <>
      {value.type === "fill_blank" ? (
        <BlankPromptField
          value={value}
          onChange={onChange}
          onEditor={setPromptEditor}
          placeholder={placeholder}
          label={label}
        />
      ) : (
        <QuestionProseField
          id="question-prompt"
          text={value.prompt}
          content={value.promptContent}
          label={label}
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
        ai={ai}
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
