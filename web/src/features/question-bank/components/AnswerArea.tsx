import { BlanksEditor } from "./BlanksEditor";
import { GradingNote } from "./GradingNote";
import { OptionsEditor } from "./OptionsEditor";
import { SampleAnswerField } from "./SampleAnswerField";
import { TrueFalseField } from "./TrueFalseField";
import type { QuestionValues } from "../questionSchema";

/**
 * AnswerArea is the block a question's type answers with, then its grading
 * note. It holds no state: every block reads and writes `value`, so swapping
 * questions or types loses nothing. `onInsertGap` is the prompt's own gap
 * command, for the fill-in-the-blank block's empty state.
 */
export function AnswerArea({
  value,
  onChange,
}: Readonly<{
  value: QuestionValues;
  onChange: (value: QuestionValues) => void;
  onInsertGap?: () => void;
}>) {
  return (
    <div data-answer-area="" className="flex flex-col gap-2.5">
      <AnswerBlock value={value} onChange={onChange} />
      <GradingNote type={value.type} />
    </div>
  );
}

function AnswerBlock({
  value,
  onChange,
}: Readonly<{ value: QuestionValues; onChange: (value: QuestionValues) => void }>) {
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
      return (
        <BlanksEditor
          prompt={value.prompt}
          content={value.promptContent}
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
