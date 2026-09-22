import { useState, type SyntheticEvent } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { cn } from "@/lib/utils";
import { format, isComplete, normalize } from "../code";

/**
 * S-01's code field: one input, not eight boxes, formatted as you type. The
 * join page uses it at full size; S-17's classes panel uses it in the panel.
 * Submitting hands over the canonical code; the confirm step is the caller's.
 */
export function JoinCodeForm({
  id,
  initial = "",
  size = "lg",
  onContinue,
}: Readonly<{
  id: string;
  initial?: string;
  size?: "lg" | "default";
  onContinue: (code: string) => void;
}>) {
  const { t } = useTranslation();
  const [code, setCode] = useState(() => format(initial));
  const [error, setError] = useState<string | null>(null);

  function onSubmit(event: SyntheticEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!isComplete(code)) {
      setError(t("join.errors.incomplete"));
      return;
    }
    onContinue(normalize(code));
  }

  return (
    <form onSubmit={onSubmit} noValidate>
      <Label htmlFor={id}>{t("join.codeLabel")}</Label>
      <Input
        id={id}
        className={cn(
          "mt-1.5 h-11 font-mono tracking-wide",
          size === "lg" && "text-lg lg:text-lg",
        )}
        value={code}
        onChange={(e) => {
          setCode(format(e.target.value));
          setError(null);
        }}
        autoCapitalize="characters"
        autoComplete="off"
        spellCheck={false}
        inputMode="text"
        maxLength={9}
        placeholder={size === "lg" ? undefined : "K7M3-P9QR"}
        aria-describedby={`${id}-hint`}
        aria-invalid={error ? true : undefined}
      />
      <p id={`${id}-hint`} className="text-muted-foreground mt-1.5 text-xs">
        {t("join.codeHint")}
      </p>

      {error ? (
        <p role="alert" className="text-destructive mt-1.5 text-xs">
          {error}
        </p>
      ) : null}

      <Button
        type="submit"
        size={size}
        className={cn("w-full", size === "lg" ? "mt-5" : "mt-3")}
        disabled={!isComplete(code)}
      >
        {t("join.continue")}
      </Button>
    </form>
  );
}
