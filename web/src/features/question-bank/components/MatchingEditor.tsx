import { useId, useRef } from "react";
import { useTranslation } from "react-i18next";
import { ArrowRight, Plus, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { toast } from "@/components/ui/sonner";
import { Switch } from "@/components/ui/switch";
import { cn } from "@/lib/utils";
import { matchItemKey, matchOptionKey } from "../matchKeys";
import {
  MAX_MATCH_ITEMS,
  MAX_MATCH_OPTIONS,
  MIN_MATCH_ITEMS,
  matchingProblems,
  type QuestionValues,
} from "../questionSchema";

type Option = QuestionValues["options"][number];
type Item = NonNullable<QuestionValues["matchItems"]>[number];

const UNSET = "";
/** MIN_MATCH_OPTIONS is the fewest options a matching question takes, as for a choice question. */
const MIN_MATCH_OPTIONS = 2;
/** CHOOSE is the list's "Choose…" entry, which returns an item to unset (Radix allows no empty value). */
const CHOOSE = "-1";

/** A row's frame and its bare input, as the builder draws a matching row. */
const ROW =
  "border-input bg-background flex items-center gap-2 rounded-[9px] border py-1 pr-1.5 pl-2";
const BARE =
  "h-8 min-w-0 flex-1 rounded-[6px] border-transparent bg-transparent px-px shadow-none lg:text-[14px] dark:bg-transparent";
const KEY = "text-muted-fg flex-none text-[12.5px] font-semibold";
const ADD =
  "text-muted-fg h-7.5 self-start rounded-[7px] px-2.5 text-[12.5px] font-normal";

/**
 * MatchingEditor is a matching question's answer, as the builder draws it
 * (2026-10-04): the items, lettered A, B, C…, each with the option it matches
 * from a select ("Choose…", then i, ii, iii…), beside the options students
 * choose from, numbered i to xii, an option no item uses tagged "Extra". Two
 * columns from 1100px of page width, one below. Items run 2 to 20 and options
 * 2 to 12, and the toasts name those bounds, not the deck's ten (DG-113).
 * Removing an option unsets the items that used it. An item with no option
 * chosen has the danger border. "Each choice can be used more than once" is
 * not drawn and is listed for sign-off; while it is off, an option a second
 * item takes is marked on that item too.
 */
export function MatchingEditor({
  value,
  onChange,
}: Readonly<{ value: QuestionValues; onChange: (value: QuestionValues) => void }>) {
  const { t } = useTranslation();
  const itemsHeading = useId();
  const optionsHeading = useId();
  const note = useId();
  const addItemButton = useRef<HTMLButtonElement>(null);
  const addOptionButton = useRef<HTMLButtonElement>(null);
  const items = value.matchItems ?? [];
  const options = value.options;
  const reuse = value.matchAllowReuse === true;
  const problems = matchingProblems(value);
  const used = new Set(items.map((item) => item.correctOption));

  function setItems(next: Item[]) {
    onChange({ ...value, matchItems: next });
  }

  function setOptions(next: Option[], nextItems: Item[] = items) {
    onChange({ ...value, options: next, matchItems: nextItems });
  }

  function addItem() {
    if (items.length >= MAX_MATCH_ITEMS) {
      toast.error(
        t("questionEditor.matching.maxItemsToast", { count: MAX_MATCH_ITEMS }),
        { id: "matching-items-bound" },
      );
      return;
    }
    setItems([...items, { id: null, text: "", correctOption: -1 }]);
  }

  function addOption() {
    if (options.length >= MAX_MATCH_OPTIONS) {
      toast.error(t("questionEditor.maxOptionsToast", { count: MAX_MATCH_OPTIONS }), {
        id: "matching-options-bound",
      });
      return;
    }
    setOptions([...options, { id: null, text: "", isCorrect: false }]);
  }

  function removeItem(index: number) {
    if (items.length <= MIN_MATCH_ITEMS) {
      toast.error(
        t("questionEditor.matching.minItemsToast", { count: MIN_MATCH_ITEMS }),
        { id: "matching-items-bound" },
      );
      return;
    }
    setItems(items.filter((_, i) => i !== index));
    addItemButton.current?.focus();
  }

  function removeOption(index: number) {
    if (options.length <= MIN_MATCH_OPTIONS) {
      toast.error(
        t("questionEditor.matching.minOptionsToast", { count: MIN_MATCH_OPTIONS }),
        { id: "matching-options-bound" },
      );
      return;
    }
    addOptionButton.current?.focus();
    setOptions(
      options.filter((_, i) => i !== index),
      items.map((item) => ({
        ...item,
        correctOption: shifted(item.correctOption, index),
      })),
    );
  }

  const sentence = problemSentence(problems);

  return (
    <div className="flex flex-col gap-3">
      <div className="grid grid-cols-[minmax(0,1fr)] gap-x-4 gap-y-3.5 min-[1100px]:grid-cols-[repeat(2,minmax(0,1fr))]">
        <div className="flex min-w-0 flex-col gap-2">
          <p id={itemsHeading} className="text-[12.5px] font-medium">
            {t("questionEditor.matching.items")}{" "}
            <span className="text-muted-fg font-normal">
              · {t("questionEditor.matching.itemsHint")}
            </span>
          </p>
          <div
            role="group"
            aria-labelledby={itemsHeading}
            aria-describedby={sentence ? note : undefined}
            className="flex flex-col gap-2"
          >
            {items.map((item, index) => {
              const key = matchItemKey(index);
              const unset = problems.unset.has(index);
              const taken = problems.taken.has(index);
              return (
                <div key={item.id ?? `new-${index}`} className={ROW}>
                  <span aria-hidden="true" className={cn(KEY, "min-w-4")}>
                    {key}
                  </span>
                  <Input
                    value={item.text}
                    aria-label={t("questionEditor.matching.item", { key })}
                    placeholder={t("questionEditor.matching.itemPlaceholder")}
                    className={BARE}
                    onChange={(event) =>
                      setItems(
                        items.map((current, i) =>
                          i === index
                            ? { ...current, text: event.target.value }
                            : current,
                        ),
                      )
                    }
                  />
                  <ArrowRight
                    aria-hidden="true"
                    className="text-muted-fg size-3.5 flex-none"
                  />
                  <Select
                    value={unset ? UNSET : String(item.correctOption)}
                    onValueChange={(next) =>
                      setItems(
                        items.map((current, i) =>
                          i === index
                            ? { ...current, correctOption: Number(next) }
                            : current,
                        ),
                      )
                    }
                  >
                    <SelectTrigger
                      size="sm"
                      aria-label={t("questionEditor.matching.correctFor", { key })}
                      aria-invalid={unset || taken}
                      className={cn(
                        "bg-card dark:bg-card h-7.5 max-w-30 flex-none rounded-[7px] px-2 text-[12.5px]",
                        unset || taken ? "border-danger" : "border-border",
                      )}
                    >
                      <SelectValue placeholder={t("questionEditor.matching.choose")}>
                        {unset ? null : matchOptionKey(item.correctOption)}
                      </SelectValue>
                    </SelectTrigger>
                    <SelectContent align="end">
                      <SelectItem value={CHOOSE}>
                        {t("questionEditor.matching.choose")}
                      </SelectItem>
                      {options.map((option, j) => (
                        <SelectItem key={option.id ?? `new-${j}`} value={String(j)}>
                          {matchOptionKey(j)}
                          {/* The space keeps "iii Trees" two words for a screen reader (QC-R6AC-10). */}
                          {option.text.trim() === "" ? null : " "}
                          {option.text.trim() === "" ? null : (
                            <span className="text-muted-fg ml-2 max-w-48 truncate">
                              {option.text}
                            </span>
                          )}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon-sm"
                    className="text-muted-fg size-7 flex-none rounded-[6px]"
                    aria-label={t("questionEditor.matching.removeItem", { key })}
                    aria-disabled={items.length <= MIN_MATCH_ITEMS || undefined}
                    onClick={() => removeItem(index)}
                  >
                    <X aria-hidden="true" className="size-3.5" />
                  </Button>
                </div>
              );
            })}
          </div>
          <Button
            type="button"
            variant="ghost"
            size="sm"
            ref={addItemButton}
            className={ADD}
            onClick={addItem}
          >
            <Plus aria-hidden="true" className="size-3.5" />
            {t("questionEditor.matching.addItem")}
          </Button>
        </div>

        <div className="flex min-w-0 flex-col gap-2">
          <p id={optionsHeading} className="text-[12.5px] font-medium">
            {t("questionEditor.options")}{" "}
            <span className="text-muted-fg font-normal">
              · {t("questionEditor.matching.optionsHint")}
            </span>
          </p>
          <div
            role="group"
            aria-labelledby={optionsHeading}
            className="flex flex-col gap-2"
          >
            {options.map((option, index) => {
              const key = matchOptionKey(index);
              return (
                <div key={option.id ?? `new-${index}`} className={ROW}>
                  <span aria-hidden="true" className={cn(KEY, "min-w-5.5")}>
                    {key}
                  </span>
                  <Input
                    value={option.text}
                    aria-label={t("questionEditor.matching.option", { key })}
                    placeholder={t("questionEditor.matching.optionPlaceholder")}
                    className={BARE}
                    onChange={(event) =>
                      setOptions(
                        options.map((current, i) =>
                          i === index
                            ? {
                                id: current.id,
                                text: event.target.value,
                                isCorrect: false,
                              }
                            : current,
                        ),
                      )
                    }
                  />
                  {used.has(index) ? null : (
                    <span
                      title={t("questionEditor.matching.extraTitle")}
                      className="text-muted-fg flex-none text-[11.5px] whitespace-nowrap"
                    >
                      {t("questionEditor.matching.extra")}
                    </span>
                  )}
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon-sm"
                    className="text-muted-fg size-7 flex-none rounded-[6px]"
                    aria-label={t("questionEditor.matching.removeOption", { key })}
                    aria-disabled={options.length <= MIN_MATCH_OPTIONS || undefined}
                    onClick={() => removeOption(index)}
                  >
                    <X aria-hidden="true" className="size-3.5" />
                  </Button>
                </div>
              );
            })}
          </div>
          <Button
            type="button"
            variant="ghost"
            size="sm"
            ref={addOptionButton}
            className={ADD}
            onClick={addOption}
          >
            <Plus aria-hidden="true" className="size-3.5" />
            {t("questionEditor.addOption")}
          </Button>
        </div>
      </div>

      <label className="flex cursor-pointer items-center gap-2.5 text-[13px]">
        <Switch
          size="sm"
          checked={reuse}
          onCheckedChange={(on) => onChange({ ...value, matchAllowReuse: on })}
        />
        {t("questionEditor.matching.reuse")}
      </label>

      {sentence ? (
        <p id={note} className="text-danger-ink text-xs leading-normal">
          {t(sentence)}
        </p>
      ) : null}
    </div>
  );
}

/** shifted is an item's option index once the option at `removed` is gone: unset if it was that one. */
function shifted(index: number, removed: number): number {
  if (index === removed) return -1;
  return index > removed ? index - 1 : index;
}

function problemSentence(problems: ReturnType<typeof matchingProblems>): string | null {
  if (problems.moreItemsThanOptions)
    return "questionEditor.errors.matchMoreItemsThanOptions";
  if (problems.taken.size) return "questionEditor.errors.matchOptionTaken";
  return null;
}
