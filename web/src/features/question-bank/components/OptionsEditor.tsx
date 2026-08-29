import { useTranslation } from "react-i18next";
import { Plus, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import type { QuestionValues } from "@/features/question-bank/questionSchema";

type Option = QuestionValues["options"][number];

interface OptionsEditorProps {
  options: Option[];
  multiple: boolean;
  fixed: boolean;
  onChange: (options: Option[]) => void;
}

/**
 * The deck's A-04 options list: the correct answer is marked where the option is
 * written, not in a separate answer-key panel, because two places to look is how
 * a test ships with the wrong key.
 *
 * `fixed` is true/false, which has exactly two options a teacher never renames.
 */
export function OptionsEditor({
  options,
  multiple,
  fixed,
  onChange,
}: OptionsEditorProps) {
  const { t } = useTranslation();

  function setCorrect(index: number, correct: boolean) {
    onChange(
      options.map((option, i) => ({
        ...option,
        // Single choice is exclusive, so marking one unmarks the rest.
        isCorrect: multiple ? (i === index ? correct : option.isCorrect) : i === index,
      })),
    );
  }

  return (
    <div>
      <div className="mb-2 flex items-center justify-between">
        <span className="text-[0.8125rem] font-medium">
          {t("questionEditor.options")}
        </span>
        <span className="text-muted-foreground text-xs">
          {multiple
            ? t("questionEditor.optionsHintMulti")
            : t("questionEditor.optionsHint")}
        </span>
      </div>

      <div className="space-y-2">
        {options.map((option, index) => (
          <div key={index} className="flex items-center gap-2.5">
            <input
              type={multiple ? "checkbox" : "radio"}
              name="question-option"
              checked={option.isCorrect}
              onChange={(event) => setCorrect(index, event.target.checked)}
              aria-label={t("questionEditor.optionPlaceholder", { n: index + 1 })}
              className="border-input accent-foreground size-4 shrink-0"
            />
            <Input
              value={option.text}
              placeholder={t("questionEditor.optionPlaceholder", { n: index + 1 })}
              onChange={(event) =>
                onChange(
                  options.map((current, i) =>
                    i === index ? { ...current, text: event.target.value } : current,
                  ),
                )
              }
            />
            {fixed ? null : (
              <Button
                type="button"
                variant="ghost"
                size="icon-sm"
                aria-label={t("questionEditor.removeOptionN", { n: index + 1 })}
                onClick={() => onChange(options.filter((_, i) => i !== index))}
              >
                <Trash2 aria-hidden="true" />
              </Button>
            )}
          </div>
        ))}
      </div>

      {fixed ? null : (
        <Button
          type="button"
          variant="ghost"
          size="sm"
          className="text-muted-foreground mt-2"
          onClick={() =>
            onChange([...options, { id: null, text: "", isCorrect: false }])
          }
        >
          <Plus aria-hidden="true" />
          {t("questionEditor.addOption")}
        </Button>
      )}
    </div>
  );
}
