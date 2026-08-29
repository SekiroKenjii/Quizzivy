import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Copy } from "lucide-react";
import { Button } from "@/components/ui/button";

/**
 * The one-time password, shown inline where the copy happens.
 *
 * The deck puts the "đừng đăng lên nhóm lớp" warning in this card rather than a
 * dialog for a stated reason: the most likely leak in a small practice is a
 * well-meaning teacher pasting a credential into the class group chat, and the
 * sentence has to be at the moment of the copy.
 *
 * The plaintext lives in component state and never in the query cache — the
 * same discipline JoinCodePanel uses, because a cached secret outlives the
 * screen that was allowed to show it.
 */
export function TemporaryPasswordCard({ password }: { password: string }) {
  const { t } = useTranslation();
  const [copied, setCopied] = useState(false);
  const [failed, setFailed] = useState(false);

  async function copy() {
    try {
      await navigator.clipboard.writeText(password);
      setFailed(false);
      setCopied(true);
    } catch {
      // Clipboard access is refused in plenty of ordinary situations. Saying so
      // beats a button that silently does nothing.
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
