import type { Ref } from "react";
import { Search } from "lucide-react";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";

/**
 * TOOLBAR_SEARCH are the classes for a list toolbar's search field as the deck
 * draws it: up to 300px wide, 34px high, on the card surface.
 */
export const TOOLBAR_SEARCH =
  "w-auto min-w-0 flex-[0_1_300px] [&_input]:bg-card [&_input]:border-border [&_input]:h-8.5 [&_input]:pl-8.5 [&_input]:text-sm [&_svg]:top-[9.5px] [&_svg]:size-3.75";

/**
 * SearchInput is a list-toolbar search field with icon clearance at either
 * density. `inputRef` reaches the field, for a page that returns focus to it,
 * `maxLength` is the most the field takes in UTF-16 units, and
 * `maxCharacters` in characters (code points), which also cuts a longer
 * `value` the field is given, such as one from a link.
 */
export function SearchInput({
  value,
  onChange,
  placeholder,
  className,
  dense = false,
  inputRef,
  maxLength,
  maxCharacters,
}: Readonly<{
  value: string;
  onChange: (value: string) => void;
  placeholder: string;
  className?: string;
  dense?: boolean;
  inputRef?: Ref<HTMLInputElement>;
  maxLength?: number;
  maxCharacters?: number;
}>) {
  const fit = (text: string) => {
    if (maxCharacters === undefined) return text;
    const letters = [...text];
    return letters.length > maxCharacters
      ? letters.slice(0, maxCharacters).join("")
      : text;
  };
  return (
    <div className={cn("relative w-72", className)}>
      <Search
        className={cn(
          "text-muted-foreground pointer-events-none absolute top-2.5 left-2.5 size-4",
          dense && "top-2 left-2 size-3.5",
        )}
        aria-hidden="true"
      />
      <Input
        type="search"
        className={cn(
          "pl-9 in-data-[scale=deck]:pl-9",
          dense &&
            "h-8 pl-8 text-xs in-data-[scale=deck]:h-8 in-data-[scale=deck]:pl-8 lg:text-xs",
        )}
        ref={inputRef}
        maxLength={maxLength}
        value={fit(value)}
        placeholder={placeholder}
        aria-label={placeholder}
        onChange={(event) => onChange(fit(event.target.value))}
      />
    </div>
  );
}
