import { useEffect, useId, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { X } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Label } from "@/components/ui/label";
import { LoadMoreSentinel } from "@/components/shared/LoadMoreSentinel";
import { cn } from "@/lib/utils";

export interface Token {
  id: string;
  label: string;
  hint?: string;
}

interface TokenFieldProps {
  label: string;
  optionalNote?: string;
  placeholder: string;
  selected: Token[];
  options: Token[];
  loading: boolean;
  /** More options exist beyond `options`; reaching the end of the list asks for them. */
  hasMore?: boolean;
  loadingMore?: boolean;
  onEndReached?: () => void;
  query: string;
  onQueryChange: (query: string) => void;
  onAdd: (token: Token) => void;
  onRemove: (id: string) => void;
}

/**
 * G-01's "Lớp" and "Học viên lẻ" fields: chosen things as removable chips, with
 * one combobox that searches for the next one.
 */
export function TokenField({
  label,
  optionalNote,
  placeholder,
  selected,
  options,
  loading,
  hasMore = false,
  loadingMore = false,
  onEndReached,
  query,
  onQueryChange,
  onAdd,
  onRemove,
}: Readonly<TokenFieldProps>) {
  const { t } = useTranslation();
  const inputId = useId();
  const listId = useId();
  const listRef = useRef<HTMLUListElement>(null);
  const [open, setOpen] = useState(false);
  const [activeId, setActiveId] = useState<string | null>(null);

  const chosen = new Set(selected.map((token) => token.id));
  const available: Token[] = loading
    ? []
    : options.filter((option) => !chosen.has(option.id));
  const active = available.findIndex((option) => option.id === activeId);
  const activeOption = active === -1 ? undefined : available[active];
  const optionId = (index: number) => `${listId}-option-${index}`;

  useEffect(() => {
    const list = listRef.current;
    if (!open || list === null) return;
    const option = list.querySelector<HTMLElement>(`[data-index="${active}"]`);
    if (option === null) return;
    const box = option.getBoundingClientRect();
    const view = list.getBoundingClientRect();
    if (box.top < view.top) list.scrollTop -= view.top - box.top;
    else if (box.bottom > view.bottom) list.scrollTop += box.bottom - view.bottom;
  }, [active, open]);

  function pick(token: Token) {
    onAdd(token);
    onQueryChange("");
    setActiveId(null);
  }

  function move(delta: number) {
    if (available.length === 0) return;
    let from = active;
    if (active === -1) from = delta > 0 ? -1 : 0;
    const next = (from + delta + available.length) % available.length;
    setActiveId(available[next]!.id);
  }

  function jumpTo(index: number): boolean {
    if (!open || query !== "" || available.length === 0) return false;
    setActiveId(available[index]!.id);
    return true;
  }
  function arrow(delta: 1 | -1): boolean {
    setOpen(true);
    if (open) move(delta);
    return true;
  }
  const keyActions: Record<string, () => boolean> = {
    ArrowDown: () => arrow(1),
    ArrowUp: () => arrow(-1),
    Home: () => jumpTo(0),
    End: () => jumpTo(available.length - 1),
    Enter: () => {
      if (!open || activeOption === undefined) return false;
      pick(activeOption);
      return true;
    },
    Escape: () => {
      if (!open) return false;
      setOpen(false);
      return true;
    },
    Backspace: () => {
      if (query === "" && selected.length > 0)
        onRemove(selected[selected.length - 1]!.id);
      return false;
    },
  };
  // Each action says whether it consumed the key, which is what the input must not also see.
  function onKeyDown(event: React.KeyboardEvent<HTMLInputElement>) {
    if (keyActions[event.key]?.()) event.preventDefault();
  }

  return (
    <div>
      <Label htmlFor={inputId}>
        {label}
        {optionalNote === undefined ? null : (
          <span className="text-muted-foreground font-normal"> — {optionalNote}</span>
        )}
      </Label>

      <div className="mt-1.5">
        <div className="focus-within:ring-ring flex min-h-9 flex-wrap items-center gap-1.5 rounded-md border p-2 focus-within:ring-2">
          {selected.map((token) => (
            <Badge key={token.id} variant="secondary" className="gap-1 pr-1">
              {token.label}
              {token.hint === undefined ? null : (
                <span className="text-muted-foreground tabular-nums">
                  · {token.hint}
                </span>
              )}
              <button
                type="button"
                aria-label={t("assignments.removeTarget", { name: token.label })}
                className="hover:text-foreground rounded-sm"
                onMouseDown={(event) => event.preventDefault()}
                onClick={() => onRemove(token.id)}
              >
                <X className="size-3" aria-hidden="true" />
              </button>
            </Badge>
          ))}
          <input
            id={inputId}
            role="combobox"
            aria-expanded={open}
            aria-controls={open ? listId : undefined}
            aria-autocomplete="list"
            aria-activedescendant={
              open && activeOption !== undefined ? optionId(active) : undefined
            }
            className="h-6 min-w-32 flex-1 bg-transparent p-0 text-sm outline-none"
            placeholder={placeholder}
            value={query}
            onChange={(event) => {
              setOpen(true);
              onQueryChange(event.target.value);
            }}
            onClick={() => setOpen(true)}
            onFocus={() => setOpen(true)}
            onBlur={() => setOpen(false)}
            onKeyDown={onKeyDown}
          />
        </div>

        {open ? (
          <ul
            id={listId}
            ref={listRef}
            role="listbox"
            aria-label={label}
            className="bg-popover mt-1 max-h-56 overflow-y-auto rounded-md border p-1 shadow-md"
          >
            {loading && (
              <li
                role="presentation"
                className="text-muted-foreground px-2 py-1.5 text-sm"
              >
                {t("common.loading")}
              </li>
            )}
            {!loading && available.length === 0 && (
              <li
                role="presentation"
                className="text-muted-foreground px-2 py-1.5 text-sm"
              >
                {t("assignments.noCandidates")}
              </li>
            )}
            {!loading && available.length > 0 && (
              <>
                {available.map((option, index) => (
                  <li key={option.id} role="presentation">
                    <button
                      type="button"
                      id={optionId(index)}
                      role="option"
                      aria-selected={index === active}
                      data-index={index}
                      tabIndex={-1}
                      className={cn(
                        "flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-left text-sm",
                        index === active ? "bg-secondary" : "hover:bg-secondary",
                      )}
                      onMouseDown={(event) => event.preventDefault()}
                      onClick={() => pick(option)}
                    >
                      <span className="truncate">{option.label}</span>
                      {option.hint === undefined ? null : (
                        <span className="text-muted-foreground ml-auto shrink-0 truncate text-xs">
                          {option.hint}
                        </span>
                      )}
                    </button>
                  </li>
                ))}
                <li role="presentation">
                  <LoadMoreSentinel
                    active={hasMore}
                    loading={loadingMore}
                    onVisible={() => onEndReached?.()}
                  />
                </li>
              </>
            )}
          </ul>
        ) : null}
      </div>
    </div>
  );
}
