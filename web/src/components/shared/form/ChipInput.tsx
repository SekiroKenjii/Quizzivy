import { useRef, type ReactNode } from "react";
import { X } from "lucide-react";

import { cn } from "@/lib/utils";

import { sameIgnoringCase, useChipEntry, type ChipSame } from "./chips";

type ChipTone = "neutral" | "success";
type ChipSize = "sm" | "md";

const TONES: Record<ChipTone, { chip: string; remove: string }> = {
  neutral: {
    chip: "bg-muted text-fg",
    remove: "text-muted-fg hover:bg-hover hover:text-fg",
  },
  success: { chip: "bg-success-soft text-success-ink", remove: "" },
};

const SIZES: Record<ChipSize, { chip: string; label: string; remove: string }> = {
  sm: { chip: "h-5.5 pl-2 text-xs", label: "leading-4", remove: "size-4" },
  md: { chip: "h-6 pl-2.25 text-meta", label: "leading-4.5", remove: "size-4.5" },
};

const FRAME =
  "border-input bg-bg focus-within:border-ring has-[input:focus-visible]:outline-focus min-h-9 rounded-md border px-1.5 py-1.25 has-[input:focus-visible]:outline-2 has-[input:focus-visible]:outline-offset-2";

/**
 * CHIP_INPUT is the text input of a chip field, without its height and its
 * least width: it takes the room the chips leave, has no box of its own, and
 * keeps 16px text below 1024px so that a phone does not zoom in on it.
 */
export const CHIP_INPUT =
  "placeholder:text-muted-fg flex-[1_1_140px] border-0 bg-transparent p-0 text-[length:var(--text-input)] lg:text-sm";

/**
 * ChipBox draws the values of a chip field as chips, each with a button that
 * removes it and is named by `removeLabel`, followed by `children`, which is
 * the field's input. With `frame` it is the deck's bordered field, whose
 * border and focus ring follow the input inside it; without it the row that
 * holds it draws the box. `invalid` turns the frame's border to the danger
 * tone.
 */
export function ChipBox({
  values,
  onRemove,
  removeLabel,
  tone = "neutral",
  size = "sm",
  frame = true,
  invalid = false,
  children,
}: Readonly<{
  values: readonly string[];
  onRemove: (index: number) => void;
  removeLabel: (value: string) => string;
  tone?: ChipTone | undefined;
  size?: ChipSize | undefined;
  frame?: boolean | undefined;
  invalid?: boolean | undefined;
  children: ReactNode;
}>) {
  return (
    <div
      data-slot="chip-box"
      className={cn(
        "flex flex-wrap items-center gap-1.25",
        frame ? FRAME : "min-h-7 min-w-0 flex-1",
        frame && invalid && "border-danger focus-within:border-danger",
      )}
    >
      {values.map((value, index) => (
        <span
          key={`${index}:${value}`}
          data-slot="chip"
          className={cn(
            "border-border inline-flex max-w-full items-center gap-0.5 rounded-full border pr-0.75 font-medium whitespace-nowrap",
            TONES[tone].chip,
            SIZES[size].chip,
          )}
        >
          <span className={cn("min-w-0 truncate", SIZES[size].label)}>{value}</span>
          <button
            type="button"
            aria-label={removeLabel(value)}
            className={cn(
              "grid flex-none place-items-center rounded-full",
              TONES[tone].remove,
              SIZES[size].remove,
            )}
            onClick={() => onRemove(index)}
          >
            <X aria-hidden="true" className="size-2.75" />
          </button>
        </span>
      ))}
      {children}
    </div>
  );
}

/**
 * ChipInput is a field of short free-text values, each drawn as a chip: the
 * accepted answers of a gap. Enter adds what is typed, and so do a comma
 * (unless `commaAdds` is false, for values that may hold one) and leaving the
 * field; Backspace in the empty input removes the last value. An entry is
 * trimmed and put in NFC, and one that is `same` as a value already there is
 * not added: by default the two are compared lower-cased, so accents count.
 * `label` names the input. `tone` and `size` pick the chip (`md` is the
 * builder's 24px, `sm` the 22px of a framed field).
 */
export function ChipInput({
  label,
  values,
  onChange,
  placeholder,
  removeLabel,
  tone = "neutral",
  size = "sm",
  frame = true,
  commaAdds = true,
  invalid = false,
  same = sameIgnoringCase,
}: Readonly<{
  label: string;
  values: readonly string[];
  onChange: (next: string[]) => void;
  placeholder?: string | undefined;
  removeLabel: (value: string) => string;
  tone?: ChipTone | undefined;
  size?: ChipSize | undefined;
  frame?: boolean | undefined;
  commaAdds?: boolean | undefined;
  invalid?: boolean | undefined;
  same?: ChipSame | undefined;
}>) {
  const input = useRef<HTMLInputElement>(null);
  const entry = useChipEntry({ values, onChange, same, commaAdds });
  return (
    <ChipBox
      values={values}
      removeLabel={removeLabel}
      tone={tone}
      size={size}
      frame={frame}
      invalid={invalid}
      onRemove={(index) => {
        onChange(values.filter((_, at) => at !== index));
        input.current?.focus();
      }}
    >
      <input
        ref={input}
        size={1}
        autoComplete="off"
        aria-label={label}
        aria-invalid={invalid || undefined}
        placeholder={placeholder}
        value={entry.draft}
        className={cn(
          CHIP_INPUT,
          size === "sm" ? "h-5.5 min-w-30 px-1" : "h-6.5 min-w-35",
          frame && "outline-none!",
        )}
        onChange={(event) => entry.type(event.target.value)}
        onKeyDown={entry.onKeyDown}
        onBlur={entry.commit}
      />
    </ChipBox>
  );
}
