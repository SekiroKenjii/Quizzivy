import { useEffect, useRef } from "react";
import { RadioGroup } from "radix-ui";

import { cn } from "@/lib/utils";

/**
 * RadioCardOption is one card of a RadioCards group: its value, its label, an
 * optional line under the label, and whether it can be chosen.
 */
export type RadioCardOption = {
  value: string;
  label: string;
  hint?: string | undefined;
  disabled?: boolean | undefined;
};

/** RadioCards draws a controlled, named card radio group with one tab stop and arrow selection. */
export function RadioCards({
  label,
  value,
  onChange,
  options,
  className,
}: Readonly<{
  label: string;
  value: string;
  onChange: (value: string) => void;
  options: readonly RadioCardOption[];
  className?: string | undefined;
}>) {
  const pending = useRef<{ group: HTMLElement; target: HTMLButtonElement } | null>(
    null,
  );
  const optionShape = JSON.stringify(
    options.map(({ value, disabled }) => [value, disabled === true]),
  );
  useEffect(() => {
    pending.current = null;
    return () => {
      pending.current = null;
    };
  }, [value, optionShape]);
  return (
    <RadioGroup.Root
      aria-label={label}
      value={value}
      onValueChange={(next) => {
        pending.current = null;
        onChange(next);
      }}
      onFocusCapture={(event) => {
        if (!pending.current?.target.isSameNode(event.target)) pending.current = null;
      }}
      onBlurCapture={(event) => {
        if (!pending.current?.target.isSameNode(event.relatedTarget))
          pending.current = null;
      }}
      onPointerDownCapture={() => {
        pending.current = null;
      }}
      className={cn(
        "grid grid-cols-[repeat(auto-fit,minmax(min(100%,240px),1fr))] gap-2.5",
        className,
      )}
    >
      {options.map((option) => (
        <RadioGroup.Item
          key={option.value}
          value={option.value}
          disabled={option.disabled === true}
          onKeyDown={(event) => {
            pending.current = null;
            if (
              event.target !== event.currentTarget ||
              event.currentTarget.disabled ||
              event.altKey ||
              event.ctrlKey ||
              event.metaKey ||
              event.shiftKey
            )
              return;
            const group =
              event.currentTarget.closest<HTMLElement>('[role="radiogroup"]');
            const step = arrowStep(event.key, group?.dir === "rtl");
            if (group === null || step === undefined) return;
            const items = Array.from(
              group.querySelectorAll<HTMLButtonElement>(
                '[role="radio"]:not(:disabled)',
              ),
            );
            const index = items.indexOf(event.currentTarget);
            const target = items[(index + step + items.length) % items.length];
            if (index < 0 || target === undefined) return;
            event.preventDefault();
            event.stopPropagation();
            const intent = { group, target };
            pending.current = intent;
            target.focus();
            queueMicrotask(() => {
              if (pending.current !== intent) return;
              pending.current = null;
              if (
                target.isConnected &&
                group.contains(target) &&
                document.activeElement === target &&
                !target.disabled &&
                target.getAttribute("aria-checked") !== "true"
              )
                target.click();
            });
          }}
          className="group bg-card border-border data-[state=checked]:border-primary flex items-start gap-3 rounded-lg border-[1.5px] px-3.5 py-3 text-left disabled:opacity-45"
        >
          <span
            aria-hidden="true"
            className="border-ring group-data-[state=checked]:border-primary mt-px grid size-4.5 flex-none place-items-center rounded-full border-2"
          >
            <span className="group-data-[state=checked]:bg-primary size-2 rounded-full" />
          </span>
          <span className="min-w-0">
            <span className="text-ui block leading-4.5 font-medium">
              {option.label}
            </span>
            {option.hint !== undefined && (
              <span className="text-muted-fg text-meta block">{option.hint}</span>
            )}
          </span>
        </RadioGroup.Item>
      ))}
    </RadioGroup.Root>
  );
}

function arrowStep(key: string, rtl: boolean): number | undefined {
  if (key === "ArrowDown") return 1;
  if (key === "ArrowUp") return -1;
  if (key === "ArrowRight") return rtl ? -1 : 1;
  if (key === "ArrowLeft") return rtl ? 1 : -1;
  return undefined;
}
