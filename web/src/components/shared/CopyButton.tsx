import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Check, Copy } from "lucide-react";

import { Button } from "@/components/ui/button";

/**
 * CopyButton copies `value` to the clipboard and reads "Copied" for two
 * seconds, announced politely. When the clipboard refuses, it says so with
 * `failedMessage`, which should tell the reader how to copy by hand.
 */
export function CopyButton({
  value,
  failedMessage,
}: Readonly<{ value: string; failedMessage: string }>) {
  const { t } = useTranslation();
  const [state, setState] = useState<"idle" | "copied" | "failed">("idle");

  useEffect(() => {
    if (state !== "copied") return;
    const timer = setTimeout(() => setState("idle"), 2000);
    return () => clearTimeout(timer);
  }, [state]);

  async function copy() {
    try {
      await navigator.clipboard.writeText(value);
      setState("copied");
    } catch {
      setState("failed");
    }
  }

  const copied = state === "copied";
  return (
    <>
      <Button
        type="button"
        variant="ghost"
        size="xs"
        className="hover:bg-hover gap-[5px] px-2 text-xs font-medium [&_svg:not([class*='size-'])]:size-[13px]"
        onClick={() => void copy()}
      >
        {copied ? <Check aria-hidden="true" /> : <Copy aria-hidden="true" />}
        {t(copied ? "copy.copied" : "copy.copy")}
      </Button>
      <span role="status" className="sr-only">
        {copied ? t("copy.copied") : ""}
      </span>
      {state === "failed" && (
        <p role="alert" className="text-muted-fg text-meta basis-full text-center">
          {failedMessage}
        </p>
      )}
    </>
  );
}
