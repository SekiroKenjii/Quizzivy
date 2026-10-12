import { BlanksEditor } from "./BlanksEditor";
import { GapAnswers } from "./GapAnswers";
import { GradingNote } from "./GradingNote";
import { OptionsEditor } from "./OptionsEditor";
import { SampleAnswerField } from "./SampleAnswerField";
import { TrueFalseField } from "./TrueFalseField";
import type { QuestionValues } from "../questionSchema";

/**
 * AnswerArea is the block a question's type answers with, then its grading
 * note. It holds no state: every block reads and writes `value`, so swapping
 * questions or types loses nothing. A fill-in-the-blank prompt answers with
 * GapAnswers while it is rich text: stored as rich content, or open in the
 * rich editor, which is when the host passes `onInsertGap`, the prompt's own
 * gap command for GapAnswers' empty card. A prompt in Markdown keeps
 * BlanksEditor.
 */
export function AnswerArea({
  value,
  onChange,
  onInsertGap,
}: Readonly<{
  value: QuestionValues;
  onChange: (value: QuestionValues) => void;
  onInsertGap?: (() => void) | undefined;
}>) {
  return (
    <div data-answer-area="" className="flex flex-col gap-2.5">
      <AnswerBlock value={value} onChange={onChange} onInsertGap={onInsertGap} />
      <GradingNote type={value.type} />
    </div>
  );
}

function AnswerBlock({
  value,
  onChange,
  onInsertGap,
}: Readonly<{
  value: QuestionValues;
  onChange: (value: QuestionValues) => void;
  onInsertGap?: (() => void) | undefined;
}>) {
  switch (value.type) {
    case "single_choice":
    case "multiple_choice":
      return (
        <OptionsEditor
          key={value.type}
          options={value.options}
          multiple={value.type === "multiple_choice"}
          onChange={(options) => onChange({ ...value, options })}
        />
      );
    case "true_false":
      return (
        <TrueFalseField
          options={value.options}
          onChange={(options) => onChange({ ...value, options })}
        />
      );
    case "fill_blank":
      return value.promptContent != null || onInsertGap ? (
        <GapAnswers
          content={value.promptContent ?? null}
          blanks={value.blanks}
          onChange={(blanks) => onChange({ ...value, blanks })}
          onInsertGap={onInsertGap}
        />
      ) : (
        <BlanksEditor
          prompt={value.prompt}
          blanks={value.blanks}
          onChange={(blanks) => onChange({ ...value, blanks })}
        />
      );
    case "short_answer":
      return (
        <SampleAnswerField
          value={value.sampleAnswer}
          onChange={(sampleAnswer) => onChange({ ...value, sampleAnswer })}
        />
      );
  }
}
