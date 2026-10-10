import { useId, useMemo, useState, type KeyboardEvent } from "react";
import { useTranslation } from "react-i18next";
import { Check, Search, X } from "lucide-react";
import { Segmented } from "@/components/ui/segmented";
import { fold } from "@/lib/fold";
import { cn } from "@/lib/utils";
import type { TagMatch } from "./bankFilters";

const BOX =
  "bg-card flex h-8 items-center gap-1.75 rounded-lg border px-2 focus-within:border-ring";

const CHIP =
  "bg-muted text-fg inline-flex h-5.5 items-center gap-0.5 rounded-full border pr-0.75 pl-2 text-xs leading-none font-medium whitespace-nowrap";

const CHIP_REMOVE =
  "text-muted-fg hover:bg-hover hover:text-fg grid size-4 flex-none cursor-pointer place-items-center rounded-full";

const BADGE =
  "bg-primary text-primary-fg text-2xs grid h-4.5 min-w-4.5 place-items-center rounded-full px-1.25 leading-none font-semibold tabular-nums";

function matching(
  options: readonly string[],
  selected: ReadonlySet<string>,
  query: string,
): string[] {
  const needle = fold(query.trim());
  const all = [...new Set([...selected, ...options])].sort((a, b) =>
    a.localeCompare(b, "vi"),
  );
  const found = needle === "" ? all : all.filter((tag) => fold(tag).includes(needle));
  return [
    ...found.filter((tag) => selected.has(tag)),
    ...found.filter((tag) => !selected.has(tag)),
  ];
}

/**
 * BankTagFilter is the bank's tag filter as the deck's aside draws it: a
 * search box whose list toggles a tag, the chosen tags as chips that remove
 * themselves, Clear, and Match Any | All once two tags are chosen. The box
 * is a combobox: the arrow keys move through the list, Enter toggles the
 * active tag or the first match and empties the box, and Esc closes the
 * list. The list closes when the box loses focus. Tags carry no counts,
 * because the contract has none per tag.
 */
export function BankTagFilter({
  options,
  selected,
  match,
  onSelected,
  onMatch,
}: Readonly<{
  options: readonly string[];
  selected: readonly string[];
  match: TagMatch;
  onSelected: (next: readonly string[]) => void;
  onMatch: (next: TagMatch) => void;
}>) {
  const { t } = useTranslation();
  const id = useId();
  const [query, setQuery] = useState("");
  const [open, setOpen] = useState(false);
  const [active, setActive] = useState(-1);
  const chosen = useMemo(() => new Set(selected), [selected]);
  const shown = useMemo(
    () => matching(options, chosen, query),
    [options, chosen, query],
  );
  const listId = `${id}-tags`;
  const optionId = (index: number) => `${id}-tag-${index}`;

  const toggle = (tag: string) =>
    onSelected(
      chosen.has(tag) ? selected.filter((item) => item !== tag) : [...selected, tag],
    );

  const close = () => {
    setOpen(false);
    setActive(-1);
  };

  const onKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key === "ArrowDown" || event.key === "ArrowUp") {
      event.preventDefault();
      setOpen(true);
      if (shown.length === 0) return;
      const step = event.key === "ArrowDown" ? 1 : -1;
      setActive((current) => (current + step + shown.length) % shown.length);
    } else if (event.key === "Enter") {
      event.preventDefault();
      const tag = shown[active] ?? shown[0];
      if (tag === undefined) return;
      toggle(tag);
      setQuery("");
      setActive(-1);
    } else if (event.key === "Escape" && open) {
      event.preventDefault();
      event.stopPropagation();
      close();
    }
  };

  return (
    <div className="min-w-0">
      <div className="mb-1.5 flex items-center justify-between gap-2">
        <label
          htmlFor={`${id}-search`}
          className="text-meta leading-normal font-semibold"
        >
          {t("bank.tagFilter")}
        </label>
        {selected.length > 0 && (
          <button
            type="button"
            onClick={() => onSelected([])}
            className="text-muted-fg hover:text-fg cursor-pointer text-xs leading-normal"
          >
            {t("bank.tagClear")}
          </button>
        )}
      </div>
      <div className="relative">
        <div className={BOX}>
          <Search aria-hidden="true" className="text-muted-fg size-3.5 flex-none" />
          <input
            id={`${id}-search`}
            size={1}
            role="combobox"
            aria-expanded={open}
            aria-controls={listId}
            aria-autocomplete="list"
            aria-activedescendant={open && active >= 0 ? optionId(active) : undefined}
            value={query}
            placeholder={t("bank.tagSearch")}
            data-local-escape={open || undefined}
            onFocus={() => setOpen(true)}
            onBlur={() => {
              close();
              setQuery("");
            }}
            onChange={(event) => {
              setQuery(event.target.value);
              setActive(-1);
              setOpen(true);
            }}
            onKeyDown={onKeyDown}
            className="min-w-0 flex-1 border-0 bg-transparent text-sm outline-none"
          />
          {selected.length > 0 && (
            <span aria-hidden="true" className={BADGE}>
              {selected.length}
            </span>
          )}
        </div>
        {open && (
          <div className="bg-card shadow-float absolute inset-x-0 top-9 z-(--z-popover) max-h-65 overflow-y-auto rounded-[10px] border p-1">
            <div
              id={listId}
              role="listbox"
              aria-multiselectable="true"
              aria-label={t("bank.tagFilter")}
            >
              {shown.map((tag, index) => (
                <div
                  key={tag}
                  id={optionId(index)}
                  role="option"
                  aria-selected={chosen.has(tag)}
                  tabIndex={-1}
                  onMouseDown={(event) => {
                    event.preventDefault();
                    toggle(tag);
                  }}
                  onMouseEnter={() => setActive(index)}
                  className={cn(
                    "hover:bg-hover flex cursor-pointer items-center gap-2.25 rounded-md px-2 py-1.5 text-sm leading-normal",
                    index === active && "bg-hover",
                  )}
                >
                  <span
                    aria-hidden="true"
                    className={cn(
                      "grid size-4 flex-none place-items-center rounded-[0.25rem] border",
                      chosen.has(tag)
                        ? "bg-primary border-primary text-primary-fg"
                        : "border-ring bg-card",
                    )}
                  >
                    {chosen.has(tag) ? (
                      <Check aria-hidden="true" strokeWidth={3} className="size-2.75" />
                    ) : null}
                  </span>
                  <span className="min-w-0 flex-1 truncate">{tag}</span>
                </div>
              ))}
            </div>
            {shown.length === 0 && (
              <p className="text-muted-fg text-meta px-2 py-2.5 leading-normal">
                {options.length === 0 && selected.length === 0
                  ? t("bank.noTags")
                  : t("bank.tagNoMatch", { query: query.trim() })}
              </p>
            )}
          </div>
        )}
      </div>
      {selected.length > 0 && (
        <div className="mt-2 flex flex-wrap gap-1.25">
          {selected.map((tag) => (
            <span key={tag} className={CHIP}>
              {tag}
              <button
                type="button"
                aria-label={t("bank.removeTag", { tag })}
                onClick={() => toggle(tag)}
                className={CHIP_REMOVE}
              >
                <X aria-hidden="true" className="size-2.75" />
              </button>
            </span>
          ))}
        </div>
      )}
      {selected.length > 1 && (
        <div className="text-muted-fg mt-2 flex items-center gap-2 text-xs">
          <span aria-hidden="true">{t("bank.tagMatch")}</span>
          <Segmented
            label={t("bank.tagMatchLabel")}
            size="xs"
            value={match}
            options={[
              { value: "any", label: t("bank.tagMatchAny") },
              { value: "all", label: t("bank.tagMatchAll") },
            ]}
            onChange={(value) => onMatch(value === "all" ? "all" : "any")}
          />
        </div>
      )}
    </div>
  );
}
