import { useId, useRef, useState, type KeyboardEvent } from "react";
import { useTranslation } from "react-i18next";
import { Plus, Tag } from "lucide-react";

import { cn } from "@/lib/utils";

import { CHIP_INPUT, ChipBox } from "./ChipInput";
import { sameFolded, useChipEntry, withChip } from "./chips";
import { tagOptions, type TagOption, type TagSuggestion } from "./tagOptions";

const CREATED = "\u0000";

function OptionLabel({ option }: Readonly<{ option: TagOption }>) {
  const { t } = useTranslation();
  const frame = option.create
    ? t("controls.createTag", { tag: CREATED }).split(CREATED)
    : [];
  const [start, end] = option.match ?? [0, 0];
  return (
    <>
      {option.create ? frame[0] : option.tag.slice(0, start)}
      <b className="font-semibold">
        {option.create ? option.tag : option.tag.slice(start, end)}
      </b>
      {option.create ? frame[1] : option.tag.slice(end)}
    </>
  );
}

/**
 * TagCombobox is the deck's tag field: the chosen tags as chips, an input
 * that adds one, and under it a list of the suggestions that match what is
 * typed, with the matched part in bold, then a row that creates the draft as
 * a new tag. The list is open while the input has focus and there is
 * something to list. ArrowDown and ArrowUp move through it and wrap, Enter or
 * Tab takes the active row, a press on a row takes that row, and Escape
 * closes it and keeps the draft. With the list closed, Enter adds the draft,
 * and so do a comma and leaving the field; Backspace in the empty input
 * removes the last tag. A tag is added once: two that differ only in case or
 * accents are the same tag, and an entry that equals a suggestion that way is
 * added in the suggestion's spelling. `label` names the input.
 */
export function TagCombobox({
  label,
  tags,
  onChange,
  suggestions,
  placeholder,
}: Readonly<{
  label: string;
  tags: readonly string[];
  onChange: (tags: string[]) => void;
  suggestions: readonly TagSuggestion[];
  placeholder?: string | undefined;
}>) {
  const { t } = useTranslation();
  const id = useId();
  const input = useRef<HTMLInputElement>(null);
  const [listening, setListening] = useState(false);
  const [active, setActive] = useState(0);
  const entry = useChipEntry({
    values: tags,
    onChange,
    same: sameFolded,
    commaAdds: true,
    resolve: (text) =>
      suggestions.find((suggestion) => sameFolded(suggestion.tag, text))?.tag ?? text,
  });
  const options = tagOptions(entry.draft, tags, suggestions);
  const open = listening && options.length > 0;
  const current = Math.min(active, options.length - 1);

  function take(option: TagOption) {
    const next = withChip(tags, option.tag, sameFolded);
    if (next !== null) onChange(next);
    entry.setDraft("");
    setActive(0);
  }

  function onKeyDown(event: KeyboardEvent<HTMLInputElement>) {
    const option = open ? options[current] : undefined;
    if (
      option !== undefined &&
      (event.key === "ArrowDown" || event.key === "ArrowUp")
    ) {
      event.preventDefault();
      const move = event.key === "ArrowDown" ? 1 : -1;
      setActive((current + move + options.length) % options.length);
      return;
    }
    if (
      option !== undefined &&
      (event.key === "Enter" || event.key === "Tab") &&
      !event.nativeEvent.isComposing
    ) {
      event.preventDefault();
      take(option);
      return;
    }
    if (event.key === "Escape") {
      setListening(false);
      return;
    }
    entry.onKeyDown(event);
  }

  return (
    <div className="relative">
      <ChipBox
        values={tags}
        removeLabel={(tag) => t("controls.removeTag", { tag })}
        onRemove={(index) => {
          onChange(tags.filter((_, at) => at !== index));
          input.current?.focus();
        }}
      >
        <input
          ref={input}
          size={1}
          autoComplete="off"
          role="combobox"
          aria-label={label}
          aria-expanded={open}
          data-local-escape={open || undefined}
          aria-controls={id}
          aria-autocomplete="list"
          aria-activedescendant={open ? `${id}-${current}` : undefined}
          placeholder={placeholder ?? t("controls.addTag")}
          value={entry.draft}
          className={cn(
            CHIP_INPUT,
            "h-5.5 min-w-20 flex-[1_1_80px] px-1 outline-none!",
          )}
          onChange={(event) => {
            setListening(true);
            setActive(0);
            entry.type(event.target.value);
          }}
          onKeyDown={onKeyDown}
          onFocus={() => setListening(true)}
          onBlur={() => {
            setListening(false);
            entry.commit();
          }}
        />
      </ChipBox>
      {open && (
        <div
          data-slot="tag-list"
          className="bg-card shadow-float absolute inset-x-0 top-[calc(100%+4px)] z-(--z-popover) rounded-lg border p-1"
        >
          <div
            id={id}
            role="listbox"
            aria-label={t("controls.tagSuggestions")}
            className="flex flex-col gap-px"
          >
            {options.map((option, index) => {
              const Icon = option.create ? Plus : Tag;
              const meta = option.create ? t("controls.newTag") : option.meta;
              return (
                <div
                  key={option.create ? CREATED : option.tag}
                  id={`${id}-${index}`}
                  role="option"
                  tabIndex={-1}
                  aria-selected={index === current}
                  className={cn(
                    "text-fg flex min-h-8 items-center gap-2 rounded-sm px-2 py-1.5 text-sm leading-4",
                    index === current && "bg-hover",
                  )}
                  onMouseDown={(event) => {
                    event.preventDefault();
                    take(option);
                  }}
                  onMouseEnter={() => setActive(index)}
                >
                  <Icon
                    aria-hidden="true"
                    className="text-muted-fg size-3.25 flex-none"
                  />
                  <span className="min-w-0 flex-1 truncate">
                    <OptionLabel option={option} />
                  </span>
                  {meta !== undefined && (
                    <span className="text-muted-fg text-caption leading-[0.9375rem] whitespace-nowrap">
                      {meta}
                    </span>
                  )}
                </div>
              );
            })}
          </div>
          <p className="text-muted-fg text-caption mt-0.75 border-t px-2 pt-1.5 pb-1 leading-normal">
            {t("controls.tagKeys")}
          </p>
        </div>
      )}
    </div>
  );
}
