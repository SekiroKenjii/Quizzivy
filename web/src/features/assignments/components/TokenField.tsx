import { useId, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { X } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Label } from "@/components/ui/label";

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
  query: string;
  onQueryChange: (query: string) => void;
  onAdd: (token: Token) => void;
  onRemove: (id: string) => void;
}

/**
 * G-01's "Lớp" and "Học viên lẻ" fields: chosen things as removable chips, with
 * one input that searches for the next one.
 *
 * Presentational on purpose. Where the options come from differs per field --
 * classes are one cached list, students are a paged search -- so each picker
 * owns its own query and this owns only the interaction.
 */
export function TokenField({
  label,
  optionalNote,
  placeholder,
  selected,
  options,
  loading,
  query,
  onQueryChange,
  onAdd,
  onRemove,
}: TokenFieldProps) {
  const { t } = useTranslation();
  const inputId = useId();
  const listId = useId();
  const [focused, setFocused] = useState(false);
  const blurTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  const chosen = new Set(selected.map((token) => token.id));
  const available = options.filter((option) => !chosen.has(option.id));

  function pick(token: Token) {
    onAdd(token);
    onQueryChange("");
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
                onClick={() => onRemove(token.id)}
              >
                <X className="size-3" aria-hidden="true" />
              </button>
            </Badge>
          ))}
          <input
            id={inputId}
            role="combobox"
            aria-expanded={focused}
            aria-controls={listId}
            aria-autocomplete="list"
            className="h-6 min-w-32 flex-1 bg-transparent p-0 text-sm outline-none"
            placeholder={placeholder}
            value={query}
            onChange={(event) => onQueryChange(event.target.value)}
            onFocus={() => {
              if (blurTimer.current) clearTimeout(blurTimer.current);
              setFocused(true);
            }}
            // Deferred so a click on an option lands before the list unmounts:
            // mousedown fires before blur, but the click that follows does not.
            onBlur={() => {
              blurTimer.current = setTimeout(() => setFocused(false), 120);
            }}
            onKeyDown={(event) => {
              if (event.key === "Enter" && available.length > 0) {
                event.preventDefault();
                pick(available[0]!);
              }
              if (event.key === "Backspace" && query === "" && selected.length > 0) {
                onRemove(selected[selected.length - 1]!.id);
              }
            }}
          />
        </div>

        {focused ? (
          <ul
            id={listId}
            className="bg-popover mt-1 max-h-56 overflow-y-auto rounded-md border p-1 shadow-md"
          >
            {loading ? (
              <li className="text-muted-foreground px-2 py-1.5 text-sm">
                {t("common.loading")}
              </li>
            ) : available.length === 0 ? (
              <li className="text-muted-foreground px-2 py-1.5 text-sm">
                {t("assignments.noCandidates")}
              </li>
            ) : (
              available.map((option) => (
                <li key={option.id}>
                  <button
                    type="button"
                    className="hover:bg-secondary flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-left text-sm"
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
              ))
            )}
          </ul>
        ) : null}
      </div>
    </div>
  );
}
