import type { ClipboardEvent } from "react";
import { useTranslation } from "react-i18next";

import { cn } from "@/lib/utils";
import { clean, CODE_LENGTH, EXAMPLE_CODE, group } from "../code";

const TONE = {
  idle: "border-border",
  success: "border-success",
  danger: "border-danger",
} as const;

/**
 * JoinCodeField is the deck's class-code field: one large monospaced input
 * that upper-cases what is typed, drops spaces and dashes, stops at eight
 * characters and shows the dash after the fourth. `value` and `onChange` carry
 * the cleaned code without its dash; `tone` colours the border for a found or a
 * failed code, and `errorId` names the line that explains a failure.
 */
export function JoinCodeField({
  value,
  onChange,
  tone,
  errorId,
}: Readonly<{
  value: string;
  onChange: (code: string) => void;
  tone: keyof typeof TONE;
  errorId?: string | undefined;
}>) {
  const { t } = useTranslation();

  function onPaste(event: ClipboardEvent<HTMLInputElement>) {
    const field = event.currentTarget;
    const start = field.selectionStart ?? field.value.length;
    const end = field.selectionEnd ?? field.value.length;
    event.preventDefault();
    onChange(
      clean(
        field.value.slice(0, start) +
          event.clipboardData.getData("text") +
          field.value.slice(end),
      ),
    );
  }

  return (
    <input
      value={group(value)}
      onChange={(event) => onChange(clean(event.target.value))}
      onPaste={onPaste}
      aria-label={t("join.codeLabel")}
      aria-invalid={tone === "danger" || undefined}
      aria-describedby={errorId}
      placeholder={EXAMPLE_CODE}
      maxLength={CODE_LENGTH + 1}
      autoComplete="off"
      autoCapitalize="characters"
      autoCorrect="off"
      spellCheck={false}
      className={cn(
        "bg-bg text-fg placeholder:text-muted-fg/70 text-stat h-[58px] w-full rounded-xl border-[1.5px] px-3.5 text-center font-mono font-semibold tracking-[0.14em] outline-none",
        TONE[tone],
      )}
    />
  );
}
