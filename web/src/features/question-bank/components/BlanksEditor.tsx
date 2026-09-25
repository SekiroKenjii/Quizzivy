import { useState } from "react";
import { ConfirmDialog } from "@/components/shared/ConfirmDialog";
import { QuestionProse } from "@/components/shared/content/QuestionProse";
import { questionGaps } from "@/components/shared/content/gaps";
import type { QuestionPromptContent } from "@/components/shared/content/questionContent";
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
  content?: QuestionPromptContent | null | undefined;
  blanks: Blank[];
  onChange: (blanks: Blank[]) => void;
}

/** fill_blank's answer editor, with the placeholder agreement checked live. */
export function BlanksEditor({
  prompt,
  content,
  blanks,
  onChange,
}: Readonly<BlanksEditorProps>) {
  const { t } = useTranslation();
  const [removing, setRemoving] = useState<string | null>(null);
  const gaps = new Map(
    content ? questionGaps(content).map((gap) => [gap.id, gap]) : [],
  );
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
          {content == null ? (
            <Markdown plugins={[blankSlots]}>{prompt}</Markdown>
          ) : (
            <QuestionProse text={prompt} content={content} />
          )}
        </div>
      </div>

      <div className="mt-4 mb-2 flex items-center justify-between">
        <span className="text-[0.8125rem] font-medium">
          {t("questionEditor.blanks")}
        </span>
        <span className="text-muted-foreground text-xs">
          {t(
            content == null
              ? "questionEditor.blanksHint"
              : "questionEditor.richBlankAnswersHint",
          )}
        </span>
      </div>

      <div className="space-y-3">
        {blanks.map((blank, index) => (
          <div key={blank.gapId ?? index} className="space-y-2 rounded-md border p-3">
            <div className="flex items-center justify-between gap-3">
              <span className="text-[0.8125rem] font-medium">
                {t("questionEditor.blankOrdinal", { n: blank.ordinal })}
              </span>
              <Button
                type="button"
                variant="ghost"
                size="icon-sm"
                aria-label={t("questionEditor.removeBlankN", { n: blank.ordinal })}
                disabled={
                  content != null && (blank.gapId == null || gaps.has(blank.gapId))
                }
                onClick={() =>
                  content == null
                    ? onChange(renumber(blanks.filter((_, i) => i !== index)))
                    : setRemoving(blank.gapId ?? null)
                }
              >
                <Trash2 aria-hidden="true" />
              </Button>
            </div>

            {content != null && (blank.gapId == null || !gaps.has(blank.gapId)) && (
              <p role="alert" className="text-sm">
                {t("questionEditor.orphanedBlank")}
              </p>
            )}
            <Textarea
              value={blank.acceptedAnswers.join("\n")}
              aria-label={t("questionEditor.acceptedAnswersFor", { n: blank.ordinal })}
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
                aria-label={t("questionEditor.caseSensitiveFor", { n: blank.ordinal })}
              />
            </div>
          </div>
        ))}
      </div>

      {content == null && (
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
      )}

      {content == null && hasMismatch(mismatch) ? (
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
      <ConfirmDialog
        open={removing != null}
        onOpenChange={(open) => {
          if (!open) setRemoving(null);
        }}
        title={t("questionEditor.removeOrphanTitle")}
        description={t("questionEditor.removeOrphanDescription")}
        confirmLabel={t("questionEditor.removeOrphanConfirm")}
        onConfirm={() => {
          onChange(blanks.filter((blank) => blank.gapId !== removing));
          setRemoving(null);
        }}
      />
    </div>
  );
}

// Ordinals address the prompt's markers, so removing a blank renumbers the rest
// rather than leaving a gap nothing can point at.
function renumber(blanks: Blank[]): Blank[] {
  return blanks.map((blank, index) => ({ ...blank, ordinal: index + 1 }));
}
