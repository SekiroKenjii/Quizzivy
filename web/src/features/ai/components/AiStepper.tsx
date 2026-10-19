import { Minus, Plus } from "lucide-react";
import { cn } from "@/lib/utils";

const BUTTON =
  "hover:bg-hover grid w-8 flex-none place-items-center disabled:opacity-40 max-[767px]:w-11";

function clamp(digit: string, min: number, max: number) {
  return String(Math.min(max, Math.max(min, Number(digit))));
}

/**
 * AiStepper is the deck's count field: −, one digit and +, from `min` to
 * `max`. A typed digit is clamped into the range, a letter is ignored, and
 * the field may be left empty, which `blank` fills again on blur when given.
 */
export function AiStepper({
  id,
  label,
  fewer,
  more,
  value,
  min,
  max,
  blank,
  disabled = false,
  describedBy,
  onChange,
}: Readonly<{
  id?: string;
  label: string;
  fewer: string;
  more: string;
  value: string;
  min: number;
  max: number;
  blank?: string;
  disabled?: boolean;
  describedBy?: string;
  onChange: (value: string) => void;
}>) {
  const n = value === "" ? null : Number(value);
  const decOff = disabled || n === null || n <= min;
  const incOff = disabled || (n !== null && n >= max);
  return (
    <div
      role="group"
      aria-label={label}
      className={cn(
        "bg-card inline-flex h-8 items-stretch overflow-hidden rounded-[0.5625rem] border max-[767px]:h-11",
        disabled && "opacity-50",
      )}
    >
      <button
        type="button"
        aria-label={fewer}
        disabled={decOff}
        className={BUTTON}
        onClick={() => n !== null && onChange(String(n - 1))}
      >
        <Minus aria-hidden="true" className="size-3.75" />
      </button>
      <input
        id={id}
        type="text"
        inputMode="numeric"
        maxLength={2}
        aria-label={label}
        aria-describedby={describedBy}
        disabled={disabled}
        value={value}
        onChange={(event) => {
          const digit = event.target.value.replace(/\D/g, "").slice(-1);
          onChange(digit === "" ? "" : clamp(digit, min, max));
        }}
        onBlur={() => {
          if (value === "" && blank !== undefined) onChange(blank);
        }}
        className="bg-bg text-ui w-10 min-w-0 border-x text-center font-semibold tabular-nums max-[767px]:w-13 max-[767px]:text-base"
      />
      <button
        type="button"
        aria-label={more}
        disabled={incOff}
        className={BUTTON}
        onClick={() => onChange(n === null ? String(min) : String(n + 1))}
      >
        <Plus aria-hidden="true" className="size-3.75" />
      </button>
    </div>
  );
}
