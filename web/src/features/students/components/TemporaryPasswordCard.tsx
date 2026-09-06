import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Copy } from "lucide-react";
import { Button } from "@/components/ui/button";

/** The one-time password, shown inline where the copy happens. */
export function TemporaryPasswordCard({ password }: Readonly<{ password: string }>) {
  const { t } = useTranslation();
  const [copied, setCopied] = useState(false);
  const [failed, setFailed] = useState(false);

  async function copy() {
    try {
      await navigator.clipboard.writeText(password);
      setFailed(false);
      setCopied(true);
    } catch {
      // Clipboard access is refused in plenty of ordinary situations.
      setCopied(false);
      setFailed(true);
    }
  }

  return (
    <div className="bg-muted/30 space-y-2 rounded-lg border p-3.5">
      <p className="text-muted-foreground text-xs">{t("students.tempOnce")}</p>
      <div className="flex items-center gap-2">
        <code className="flex-1 font-mono text-base">{password}</code>
        <Button variant="outline" size="sm" onClick={() => void copy()}>
          <Copy aria-hidden="true" />
          {copied ? t("students.copied") : t("students.copy")}
        </Button>
      </div>
      {failed ? (
        <p role="alert" className="text-destructive text-xs">
          {t("students.copyFailed")}
        </p>
      ) : null}
      <p className="text-muted-foreground text-xs leading-relaxed">
        {t("students.tempWarning")}
      </p>
    </div>
  );
}
