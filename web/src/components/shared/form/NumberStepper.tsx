import {
  useEffect,
  useRef,
  useState,
  type KeyboardEvent,
  type MouseEvent,
} from "react";
import { useTranslation } from "react-i18next";
import { Minus, Plus } from "lucide-react";

import { cn } from "@/lib/utils";

import { clampTo, digitsOf, keyTarget, typedValue } from "./stepperValue";

const BUTTON =
  "hover:bg-muted grid size-8.5 flex-none place-items-center disabled:pointer-events-none disabled:opacity-45";

function keepFocus(event: MouseEvent) {
  event.preventDefault();
}

/**
 * NumberStepper is the deck's whole-number field: a value between a minus and
 * a plus button. The value is a spinbutton named by `label` and is the one
 * tab stop: ArrowUp and ArrowDown move it by `step`, Home and End take it to
 * `min` and `max`, and the two buttons do the same with the mouse. It can
 * also be typed: while it has focus it shows the bare number, and Enter or
 * leaving the field commits what was typed, clamped to `min` and `max` and
 * never snapped to `step`; Escape and an empty field restore the value.
 * `onChange` is called with a number inside the bounds, and only when it
 * differs from `value`. A `value` outside the bounds is drawn at the nearer
 * one. `format` writes the value with its unit, as "45 min".
 */
export function NumberStepper({
  label,
  value,
  onChange,
  min,
  max,
  step = 1,
  format = String,
  disabled = false,
}: Readonly<{
  label: string;
  value: number;
  onChange: (next: number) => void;
  min: number;
  max: number;
  step?: number | undefined;
  format?: ((value: number) => string) | undefined;
  disabled?: boolean | undefined;
}>) {
  const { t } = useTranslation();
  const input = useRef<HTMLInputElement>(null);
  const [focused, setFocused] = useState(false);
  const [draft, setDraft] = useState<string | null>(null);
  const current = clampTo(value, min, max);
  const text = format(current);
  const shown = focused ? (draft ?? String(current)) : text;

  useEffect(() => {
    if (focused) input.current?.select();
  }, [focused]);

  function move(next: number) {
    setDraft(null);
    const target = clampTo(next, min, max);
    if (target !== value) onChange(target);
  }

  function commit() {
    const typed = draft === null ? null : typedValue(draft, min, max);
    setDraft(null);
    if (typed !== null && typed !== value) onChange(typed);
  }

  function onKeyDown(event: KeyboardEvent<HTMLInputElement>) {
    if (event.key === "Escape") {
      setDraft(null);
      return;
    }
    if (event.key === "Enter") {
      if (event.nativeEvent.isComposing) return;
      event.preventDefault();
      commit();
      return;
    }
    const target = keyTarget(event.key, current, step, min, max);
    if (target === null) return;
    event.preventDefault();
    move(target);
  }

  return (
    <div
      data-slot="number-stepper"
      className="border-input bg-bg focus-within:border-ring has-[input:focus-visible]:outline-focus inline-flex h-9 items-center rounded-md border has-[input:focus-visible]:outline-2 has-[input:focus-visible]:outline-offset-2"
    >
      <button
        type="button"
        tabIndex={-1}
        aria-label={t("controls.decrease")}
        disabled={disabled || current <= min}
        className={cn(BUTTON, "rounded-l-seg")}
        onMouseDown={keepFocus}
        onClick={() => move(current - step)}
      >
        <Minus aria-hidden="true" className="size-3.5" />
      </button>
      <span className="lg:text-ui inline-grid min-w-13.5 text-[length:var(--text-input)] font-medium tabular-nums">
        <span
          aria-hidden="true"
          className="invisible col-start-1 row-start-1 px-px whitespace-pre"
        >
          {shown}
        </span>
        <input
          ref={input}
          type="text"
          inputMode="numeric"
          autoComplete="off"
          role="spinbutton"
          aria-label={label}
          aria-valuenow={current}
          aria-valuemin={min}
          aria-valuemax={max}
          aria-valuetext={text}
          size={1}
          disabled={disabled}
          value={shown}
          className="col-start-1 row-start-1 w-full min-w-0 border-0 bg-transparent p-0 text-center outline-none! disabled:opacity-45"
          onFocus={() => setFocused(true)}
          onBlur={() => {
            setFocused(false);
            commit();
          }}
          onChange={(event) => setDraft(digitsOf(event.target.value))}
          onKeyDown={onKeyDown}
        />
      </span>
      <button
        type="button"
        tabIndex={-1}
        aria-label={t("controls.increase")}
        disabled={disabled || current >= max}
        className={cn(BUTTON, "rounded-r-seg")}
        onMouseDown={keepFocus}
        onClick={() => move(current + step)}
      >
        <Plus aria-hidden="true" className="size-3.5" />
      </button>
    </div>
  );
}
