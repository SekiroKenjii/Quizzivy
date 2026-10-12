import { useId, useState } from "react";
import { useTranslation } from "react-i18next";
import { TextCursorInput, Trash2 } from "lucide-react";
import { ConfirmDialog } from "@/components/shared/ConfirmDialog";
import { ChipInput } from "@/components/shared/form/ChipInput";
import { questionGaps, type QuestionGap } from "@/components/shared/content/gaps";
import type { QuestionPromptContent } from "@/components/shared/content/questionContent";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import { cn } from "@/lib/utils";
import { gapProblems, type QuestionValues } from "../questionSchema";

type Blank = QuestionValues["blanks"][number];

/**
 * GapAnswers is a rich fill-in-the-blank prompt's answer key: one row per gap,
 * in the order the gaps stand in the prompt, holding its accepted answers as
 * chips and its "Match case" switch. With "Match case" on, two answers that
 * differ only in case are both kept, because the server grades them apart. A
 * gap with no answer is drawn in the danger tone and says so. With no gap,
 * it shows the card whose "Insert gap" runs `onInsertGap` at the prompt's
 * caret. Answers whose gap left the prompt stay, for undo, until the teacher
 * deletes them.
 */
export function GapAnswers({
  content,
  blanks,
  onChange,
  onInsertGap,
}: Readonly<{
  content: QuestionPromptContent | null;
  blanks: Blank[];
  onChange: (blanks: Blank[]) => void;
  onInsertGap?: (() => void) | undefined;
}>) {
  const { t } = useTranslation();
  const [removing, setRemoving] = useState<Blank | null>(null);
  const gaps = content == null ? [] : questionGaps(content);
  const byGap = new Map(blanks.map((blank) => [blank.gapId, blank]));
  const bound = new Set(gaps.map((gap) => gap.id));
  const orphans = blanks.filter(
    (blank) => blank.gapId == null || !bound.has(blank.gapId),
  );
  const empty = new Set(
    content == null ? [] : gapProblems(content, blanks).emptyGapLabels,
  );
  const caseMatters = gaps.some((gap) => byGap.get(gap.id)?.caseSensitive);

  function update(blank: Blank, patch: Partial<Blank>) {
    onChange(
      blanks.map((current) => (current === blank ? { ...blank, ...patch } : current)),
    );
  }

  return (
    <div className="flex flex-col gap-2">
      <p className="text-[12.5px] font-medium in-data-[field-size=page]:text-[13px] in-data-[field-size=page]:leading-[19.5px]">
        {t("questionEditor.gapAnswers.label")}{" "}
        <span className="text-muted-fg font-normal">
          · {t("questionEditor.gapAnswers.hint")}
          {caseMatters ? null : ` ${t("questionEditor.gapAnswers.caseIgnored")}`}
        </span>
      </p>

      {gaps.length === 0 ? <NoGapsCard onInsertGap={onInsertGap} /> : null}

      {gaps.map((gap) => {
        const blank = byGap.get(gap.id);
        return blank ? (
          <GapRow
            key={gap.id}
            gap={gap}
            blank={blank}
            empty={empty.has(gap.label)}
            onChange={(patch) => update(blank, patch)}
          />
        ) : null;
      })}

      {orphans.map((blank) => (
        <div
          key={blank.gapId ?? `ordinal-${blank.ordinal}`}
          role="group"
          aria-label={t("questionEditor.gapAnswers.removedGroup", {
            label: blank.ordinal,
          })}
          className="border-input flex flex-col gap-1.5 rounded-[9px] border border-dashed px-3 py-2"
        >
          <p role="alert" className="text-[12.5px] leading-normal">
            {t("questionEditor.orphanedBlank")}
          </p>
          <div className="flex flex-wrap items-center gap-2">
            {blank.acceptedAnswers.length > 0 ? (
              <span className="text-muted-fg min-w-0 text-[12.5px]">
                {blank.acceptedAnswers.join(", ")}
              </span>
            ) : null}
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className="text-muted-fg"
              onClick={() => setRemoving(blank)}
            >
              <Trash2 aria-hidden="true" />
              {t("questionEditor.gapAnswers.removeOrphan", { label: blank.ordinal })}
            </Button>
          </div>
        </div>
      ))}

      <ConfirmDialog
        open={removing != null}
        onOpenChange={(open) => {
          if (!open) setRemoving(null);
        }}
        title={t("questionEditor.removeOrphanTitle")}
        description={t("questionEditor.removeOrphanDescription")}
        confirmLabel={t("questionEditor.removeOrphanConfirm")}
        onConfirm={() => {
          onChange(blanks.filter((blank) => blank !== removing));
          setRemoving(null);
        }}
      />
    </div>
  );
}

function sameExactly(a: string, b: string): boolean {
  return a === b;
}

function GapRow({
  gap,
  blank,
  empty,
  onChange,
}: Readonly<{
  gap: QuestionGap;
  blank: Blank;
  empty: boolean;
  onChange: (patch: Partial<Blank>) => void;
}>) {
  const { t } = useTranslation();
  const message = useId();
  const label = gap.label;
  return (
    <div
      role="group"
      aria-label={t("questionEditor.gapAnswers.group", { label })}
      className="flex flex-col gap-1"
    >
      <div
        className={cn(
          "bg-background flex flex-wrap items-center gap-x-2.5 gap-y-1.5 rounded-[9px] border px-2 py-1.5",
          empty ? "border-danger" : "border-input",
        )}
      >
        <span
          aria-hidden="true"
          className="border-ring bg-muted text-muted-fg inline-flex h-6 min-w-11 flex-none items-center justify-center rounded-[6px] border-[1.5px] border-dashed px-2 text-xs leading-normal font-semibold"
        >
          {label}
        </span>
        <div className="min-w-0 flex-[1_1_220px]">
          <ChipInput
            label={t("questionEditor.gapAnswers.inputFor", { label })}
            values={blank.acceptedAnswers}
            onChange={(acceptedAnswers) => onChange({ acceptedAnswers })}
            placeholder={t(
              blank.acceptedAnswers.length === 0
                ? "questionEditor.gapAnswers.placeholderFirst"
                : "questionEditor.gapAnswers.placeholderMore",
            )}
            removeLabel={(value) =>
              t("questionEditor.gapAnswers.removeAnswer", { value })
            }
            tone="success"
            size="md"
            frame={false}
            invalid={empty}
            describedBy={empty ? message : undefined}
            same={blank.caseSensitive ? sameExactly : undefined}
          />
        </div>
        <label className="text-muted-fg flex flex-none cursor-pointer items-center gap-2 text-[12.5px]">
          {t("questionEditor.gapAnswers.matchCase")}
          <Switch
            checked={blank.caseSensitive}
            aria-label={t("questionEditor.gapAnswers.matchCaseFor", { label })}
            onCheckedChange={(caseSensitive) => onChange({ caseSensitive })}
          />
        </label>
      </div>
      {empty ? (
        <p id={message} className="text-danger-ink pl-1 text-xs leading-normal">
          {t("questionEditor.gapAnswers.empty", { label })}
        </p>
      ) : null}
    </div>
  );
}

function NoGapsCard({
  onInsertGap,
}: Readonly<{ onInsertGap?: (() => void) | undefined }>) {
  const { t } = useTranslation();
  return (
    <div className="border-border flex flex-wrap items-center gap-x-3 gap-y-2.5 rounded-[10px] border-[1.5px] border-dashed px-3.5 py-3">
      <span
        aria-hidden="true"
        className="bg-muted grid size-8.5 flex-none place-items-center rounded-[9px]"
      >
        <TextCursorInput className="size-4" />
      </span>
      <span className="min-w-0 flex-[1_1_220px]">
        <span className="block text-[13.5px] font-medium">
          {t("questionEditor.gapAnswers.noGapsTitle")}
        </span>
        <span className="text-muted-fg block text-xs leading-normal">
          {t("questionEditor.gapAnswers.noGapsText")}
        </span>
      </span>
      {onInsertGap ? (
        <Button
          type="button"
          variant="outline"
          size="sm"
          className="h-8 rounded-[8px] px-3 text-[13px] font-medium in-data-[scale=deck]:h-8 in-data-[scale=deck]:rounded-[8px] in-data-[scale=deck]:px-3 in-data-[scale=deck]:text-[13px]"
          onClick={onInsertGap}
        >
          <TextCursorInput aria-hidden="true" className="size-3.5" />
          {t("questionEditor.gapAnswers.insertGap")}
        </Button>
      ) : null}
    </div>
  );
}
