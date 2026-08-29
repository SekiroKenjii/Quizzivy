import { useTranslation } from "react-i18next";
import { Plus, Trash2 } from "lucide-react";
import { Markdown } from "@/components/shared/Markdown";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { Switch } from "@/components/ui/switch";
import { blankSlots } from "@/features/question-bank/blankSlots";
import {
  comparePlaceholders,
  hasMismatch,
} from "@/features/question-bank/placeholders";
import type { QuestionValues } from "@/features/question-bank/questionSchema";

type Blank = QuestionValues["blanks"][number];

interface BlanksEditorProps {
  prompt: string;
  blanks: Blank[];
  onChange: (blanks: Blank[]) => void;
}

/**
 * fill_blank's answer editor, with the placeholder agreement checked live.
 *
 * The server enforces the same rule at save and again at publish. Showing it
 * here is what stops a teacher discovering at publish time that the `{{3}}` they
 * typed has no blank behind it -- §8's publish gate confirms rather than
 * surprises.
 */
export function BlanksEditor({ prompt, blanks, onChange }: BlanksEditorProps) {
  const { t } = useTranslation();
  const mismatch = comparePlaceholders(
    prompt,
    blanks.map((blank) => blank.ordinal),
  );

  function update(index: number, patch: Partial<Blank>) {
    onChange(blanks.map((blank, i) => (i === index ? { ...blank, ...patch } : blank)));
  }

  return (
    <div>
      <div className="mb-2">
        <span className="text-[0.8125rem] font-medium">
          {t("questionEditor.preview")}
        </span>
        <div className="mt-1.5 rounded-md border p-3 text-base leading-relaxed">
          <Markdown plugins={[blankSlots]}>{prompt}</Markdown>
        </div>
      </div>

      <div className="mt-4 mb-2 flex items-center justify-between">
        <span className="text-[0.8125rem] font-medium">
          {t("questionEditor.blanks")}
        </span>
        <span className="text-muted-foreground text-xs">
          {t("questionEditor.blanksHint")}
        </span>
      </div>

      <div className="space-y-3">
        {blanks.map((blank, index) => (
          <div key={index} className="space-y-2 rounded-md border p-3">
            <div className="flex items-center justify-between gap-3">
              <span className="text-[0.8125rem] font-medium">
                {t("questionEditor.blankOrdinal", { n: blank.ordinal })}
              </span>
              <Button
                type="button"
                variant="ghost"
                size="icon-sm"
                aria-label={t("questionEditor.removeBlank")}
                onClick={() => onChange(renumber(blanks.filter((_, i) => i !== index)))}
              >
                <Trash2 aria-hidden="true" />
              </Button>
            </div>

            <Textarea
              value={blank.acceptedAnswers.join("\n")}
              aria-label={t("questionEditor.acceptedAnswers")}
              placeholder={t("questionEditor.acceptedAnswersHint")}
              className="min-h-16"
              onChange={(event) =>
                update(index, {
                  acceptedAnswers: event.target.value
                    .split("\n")
                    .filter((line) => line !== ""),
                })
              }
            />

            <div className="flex items-center justify-between gap-3">
              <span className="text-[0.8125rem]">
                {t("questionEditor.caseSensitive")}
              </span>
              <Switch
                checked={blank.caseSensitive}
                onCheckedChange={(checked) => update(index, { caseSensitive: checked })}
                aria-label={t("questionEditor.caseSensitive")}
              />
            </div>
          </div>
        ))}
      </div>

      <Button
        type="button"
        variant="ghost"
        size="sm"
        className="text-muted-foreground mt-2"
        onClick={() =>
          onChange([
            ...blanks,
            {
              id: null,
              ordinal: blanks.length + 1,
              acceptedAnswers: [],
              caseSensitive: false,
            },
          ])
        }
      >
        <Plus aria-hidden="true" />
        {t("questionEditor.addBlank")}
      </Button>

      {hasMismatch(mismatch) ? (
        <div role="alert" className="text-destructive mt-2 space-y-1 text-xs">
          {mismatch.missingBlanks.length > 0 ? (
            <p>
              {t("questionEditor.placeholderMissingBlank", {
                list: mismatch.missingBlanks.map((n) => `{{${n}}}`).join(", "),
              })}
            </p>
          ) : null}
          {mismatch.unreferencedBlanks.length > 0 ? (
            <p>
              {t("questionEditor.placeholderUnreferenced", {
                list: mismatch.unreferencedBlanks.join(", "),
              })}
            </p>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}

// Ordinals address the prompt's markers, so removing a blank renumbers the rest
// rather than leaving a gap nothing can point at.
function renumber(blanks: Blank[]): Blank[] {
  return blanks.map((blank, index) => ({ ...blank, ordinal: index + 1 }));
}
