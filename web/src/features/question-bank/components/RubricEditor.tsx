import { useId, useRef } from "react";
import { useTranslation } from "react-i18next";
import { ArrowDown, ArrowUp, ListChecks, Plus, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { toast } from "@/components/ui/sonner";
import { cn } from "@/lib/utils";
import {
  criterionKey,
  MAX_RUBRIC_CRITERIA,
  MAX_RUBRIC_LABEL,
  type QuestionValues,
} from "../questionSchema";

type Criterion = NonNullable<QuestionValues["rubric"]>[number];

/** IELTS_WRITING are the keys of the "IELTS Writing" preset's four criteria, in order. */
const IELTS_WRITING = ["taskResponse", "coherence", "lexical", "grammar"] as const;

const ADD = "text-muted-fg h-7.5 rounded-[7px] px-2.5 text-[12.5px] font-normal";

/**
 * ROW_ICON is a row's Move and Remove buttons. A paused one fades further
 * than the primitive's paused tone, which sits too close to the live
 * muted icons beside it (QC-R6AC-12).
 */
const ROW_ICON =
  "text-muted-fg size-7 flex-none rounded-[6px] aria-disabled:text-muted-fg/40 aria-disabled:hover:text-muted-fg/40 dark:aria-disabled:hover:text-muted-fg/40";

/**
 * RubricEditor is an essay's rubric criteria (DG-61, interim; undrawn, listed
 * for sign-off): add, rename, reorder and remove, up to six, each row named
 * by its number. With none, the hint says the grader sets the band directly,
 * and "Use IELTS Writing criteria" fills in that preset's four labels in the
 * reader's language. Two criteria with the same label, compared as the server
 * does (`criterionKey`: NFC, ignoring case and the space around them), have
 * the danger border, as the server refuses them.
 */
export function RubricEditor({
  rubric,
  onChange,
}: Readonly<{ rubric: Criterion[]; onChange: (rubric: Criterion[]) => void }>) {
  const { t } = useTranslation();
  const heading = useId();
  const hint = useId();
  const addButton = useRef<HTMLButtonElement>(null);
  const duplicates = duplicateIndexes(rubric);

  function add() {
    if (rubric.length >= MAX_RUBRIC_CRITERIA) {
      toast.error(t("questionEditor.rubric.maxToast", { count: MAX_RUBRIC_CRITERIA }), {
        id: "rubric-bound",
      });
      return;
    }
    onChange([...rubric, { id: null, label: "" }]);
  }

  function move(index: number, by: -1 | 1) {
    if (index + by < 0 || index + by >= rubric.length) return;
    const next = [...rubric];
    const [row] = next.splice(index, 1);
    if (row === undefined) return;
    next.splice(index + by, 0, row);
    onChange(next);
  }

  return (
    <div className="flex flex-col gap-2">
      <p id={heading} className="text-[12.5px] font-medium">
        {t("questionEditor.rubric.label")}{" "}
        <span className="text-muted-fg font-normal">
          · {t("questionEditor.rubric.hint", { count: MAX_RUBRIC_CRITERIA })}
        </span>
      </p>
      {rubric.length === 0 ? (
        <p id={hint} className="text-muted-fg -mt-1 text-[12.5px]">
          {t("questionEditor.rubric.none")}
        </p>
      ) : (
        <ol aria-labelledby={heading} className="flex flex-col gap-2">
          {rubric.map((criterion, index) => {
            const n = index + 1;
            const duplicate = duplicates.has(index);
            return (
              <li
                key={criterion.id ?? `new-${index}`}
                className={cn(
                  "bg-background flex items-center gap-2 rounded-[9px] border py-1 pr-1.5 pl-2",
                  duplicate ? "border-danger" : "border-input",
                )}
              >
                <span
                  aria-hidden="true"
                  className="text-muted-fg min-w-4 flex-none text-[12.5px] font-semibold"
                >
                  {n}
                </span>
                <Input
                  value={criterion.label}
                  maxLength={MAX_RUBRIC_LABEL}
                  aria-label={t("questionEditor.rubric.criterion", { n })}
                  aria-invalid={duplicate}
                  placeholder={t("questionEditor.rubric.placeholder")}
                  className="h-8 min-w-0 flex-1 rounded-[6px] border-transparent bg-transparent px-px shadow-none lg:text-[14px] dark:bg-transparent"
                  onChange={(event) =>
                    onChange(
                      rubric.map((current, i) =>
                        i === index
                          ? { ...current, label: event.target.value }
                          : current,
                      ),
                    )
                  }
                />
                <Button
                  type="button"
                  variant="ghost"
                  size="icon-sm"
                  className={ROW_ICON}
                  aria-label={t("questionEditor.rubric.moveUp", { n })}
                  aria-disabled={index === 0 || undefined}
                  onClick={() => move(index, -1)}
                >
                  <ArrowUp aria-hidden="true" className="size-3.5" />
                </Button>
                <Button
                  type="button"
                  variant="ghost"
                  size="icon-sm"
                  className={ROW_ICON}
                  aria-label={t("questionEditor.rubric.moveDown", { n })}
                  aria-disabled={index === rubric.length - 1 || undefined}
                  onClick={() => move(index, 1)}
                >
                  <ArrowDown aria-hidden="true" className="size-3.5" />
                </Button>
                <Button
                  type="button"
                  variant="ghost"
                  size="icon-sm"
                  className={ROW_ICON}
                  aria-label={t("questionEditor.rubric.remove", { n })}
                  onClick={() => {
                    onChange(rubric.filter((_, i) => i !== index));
                    addButton.current?.focus();
                  }}
                >
                  <X aria-hidden="true" className="size-3.5" />
                </Button>
              </li>
            );
          })}
        </ol>
      )}
      <div className="flex flex-wrap items-center gap-1">
        <Button
          ref={addButton}
          type="button"
          variant="ghost"
          size="sm"
          className={ADD}
          onClick={add}
        >
          <Plus aria-hidden="true" className="size-3.5" />
          {t("questionEditor.rubric.add")}
        </Button>
        {rubric.length === 0 ? (
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className={ADD}
            aria-describedby={hint}
            onClick={() =>
              onChange(
                IELTS_WRITING.map((key) => ({
                  id: null,
                  label: t(`questionEditor.rubric.ielts.${key}`),
                })),
              )
            }
          >
            <ListChecks aria-hidden="true" className="size-3.5" />
            {t("questionEditor.rubric.preset")}
          </Button>
        ) : null}
      </div>
      {duplicates.size > 0 ? (
        <p role="alert" className="text-danger-ink text-xs leading-normal">
          {t("questionEditor.errors.duplicateCriterion")}
        </p>
      ) : null}
    </div>
  );
}

function duplicateIndexes(rubric: readonly Criterion[]): ReadonlySet<number> {
  const first = new Map<string, number>();
  const out = new Set<number>();
  rubric.forEach((criterion, index) => {
    const label = criterionKey(criterion.label);
    if (label === "") return;
    const earlier = first.get(label);
    if (earlier === undefined) first.set(label, index);
    else out.add(earlier).add(index);
  });
  return out;
}
