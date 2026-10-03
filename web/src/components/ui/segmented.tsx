import { cn } from "@/lib/utils";

/**
 * Segmented is a group of buttons where exactly one is on: `role="group"`
 * with `aria-pressed` buttons, not a tab strip, because it controls no panel
 * (DG-17). On a `data-scale="deck"` surface the buttons are the deck's 30px
 * with the card fill and a 1px ring when on; `size="lg"` is the deck's 32px
 * row, used for a page's sections.
 */
export function Segmented({
  label,
  value,
  options,
  onChange,
  size = "default",
  className,
}: Readonly<{
  label: string;
  value: string;
  options: { value: string; label: string }[];
  onChange: (value: string) => void;
  size?: "default" | "lg";
  className?: string;
}>) {
  return (
    <div
      role="group"
      aria-label={label}
      className={cn(
        "bg-muted in-data-[scale=deck]:rounded-ctl inline-flex gap-0.5 rounded-lg p-[0.1875rem]",
        className,
      )}
    >
      {options.map((option) => {
        const on = option.value === value;
        return (
          <button
            key={option.value}
            type="button"
            aria-pressed={on}
            onClick={() => onChange(option.value)}
            className={cn(
              "inline-flex h-7 items-center gap-1.5 rounded-md border-0 bg-transparent px-3 text-[0.8125rem] font-medium transition-colors",
              "in-data-[scale=deck]:rounded-seg in-data-[scale=deck]:h-7.5 in-data-[scale=deck]:whitespace-nowrap",
              size === "lg" && "h-8 in-data-[scale=deck]:h-8",
              on
                ? "bg-background text-foreground shadow-card in-data-[scale=deck]:bg-card in-data-[scale=deck]:text-fg in-data-[scale=deck]:ring-border in-data-[scale=deck]:ring-1"
                : "text-muted-foreground in-data-[scale=deck]:text-muted-fg",
            )}
          >
            {option.label}
          </button>
        );
      })}
    </div>
  );
}
