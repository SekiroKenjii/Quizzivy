import { cn } from "@/lib/utils";

/**
 * The deck's `.tabs` / `.tab` (kit.css) as what it actually is: a group of
 * buttons where one is on, not a tab strip. Radix Tabs was the wrong primitive
 * here -- with no panel to control it emits `aria-controls` pointing at
 * nothing and leaves every trigger at `tabindex="-1"`, so the control cannot
 * be reached by keyboard at all.
 */
export function Segmented({
  label,
  value,
  options,
  onChange,
  className,
}: Readonly<{
  label: string;
  value: string;
  options: { value: string; label: string }[];
  onChange: (value: string) => void;
  className?: string;
}>) {
  return (
    <div
      role="group"
      aria-label={label}
      className={cn("bg-muted inline-flex gap-0.5 rounded-lg p-[0.1875rem]", className)}
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
              "focus-visible:ring-ring inline-flex h-7 items-center gap-1.5 rounded-md border-0 bg-transparent px-3 text-[0.8125rem] font-medium transition-colors focus-visible:ring-2 focus-visible:outline-none",
              on
                ? "bg-background text-foreground shadow-card"
                : "text-muted-foreground",
            )}
          >
            {option.label}
          </button>
        );
      })}
    </div>
  );
}
